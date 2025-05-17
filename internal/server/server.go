// internal/server/server.go
package server

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/AlverezYari/featherframe/internal/config"
	"github.com/AlverezYari/featherframe/internal/metrics"
	"github.com/AlverezYari/featherframe/pkg/camera"
	"github.com/gorilla/websocket"
)

type LogEntry struct {
	Timestamp time.Time
	Message   string
}

type Server struct {
	server           *http.Server
	port             string
	isRunning        bool
	logBuffer        []LogEntry
	logMutex         sync.RWMutex
	upgrader         websocket.Upgrader
	wsConnections    map[*websocket.Conn]bool
	wsConnectionsMu  sync.RWMutex
	logCallback      func(level, message string) // Callback for forwarding logs
	cameraManager    camera.CameraManager
	configuredDevice string
	appConfig        *config.AppConfig
}

func New(
	port string,
	logCallback func(level, message string),
	cameraManager camera.CameraManager,
	deviceID string,
	appConfig *config.AppConfig,
) *Server {
	return &Server{
		port:        port,
		logBuffer:   make([]LogEntry, 0, 100),
		logCallback: logCallback,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		wsConnections:    make(map[*websocket.Conn]bool),
		cameraManager:    cameraManager,
		configuredDevice: deviceID, // store the camera device
		appConfig:        appConfig,
	}
}

func (s *Server) Start() error {
	if s.isRunning {
		s.addLog("ERROR", fmt.Sprintf("Server is already running on port %s", s.port))
		return fmt.Errorf("server is already running")
	}

	mux := http.NewServeMux()
	metrics.SetStartTime(time.Now())
	// Separate handlers for streaming sites
	mux.HandleFunc("/ws/setupPreview", s.handleWebSocketPreview)
	mux.HandleFunc("/ws/liveMonitor", s.handleWebSocketLive)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "FeatherFrame Web Interface")
	})

	mux.HandleFunc("/setup-preview", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("web/templates/setup-preview.html")
		if err != nil {
			s.addLog("ERROR", fmt.Sprintf("Error parsing template: %v", err))
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	mux.HandleFunc("/live-monitor", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("web/templates/live-monitor.html")
		if err != nil {
			s.addLog("ERROR", fmt.Sprintf("Error parsing template: %v", err))
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	mux.HandleFunc("/api/screenshot", s.handleScreenshot)
	// Prometheus metrics endpoints
	mux.HandleFunc("/metrics", metrics.MetricsHandler)
	mux.HandleFunc("/api/birds_sceen", metrics.BirdsSeenHandler)
	mux.HandleFunc("/api/birds_captured", metrics.BirdsCapturedHandler)

	// Build and start the HTTP server
	s.server = &http.Server{
		Addr:    ":" + s.port,
		Handler: metrics.RequestMiddleware(mux),
	}

	go func() {
		s.addLog("INFO", fmt.Sprintf("Starting server on port %s", s.port))
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			s.addLog("ERROR", fmt.Sprintf("HTTP server error: %v", err))
		}
	}()

	s.isRunning = true
	s.addLog("INFO", fmt.Sprintf("Server is running on port %s", s.port))
	return nil
}

func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	if s.configuredDevice == "" {
		s.addLog("ERROR", "No camera configured for screenshot.")
		http.Error(w, "No camera configured", http.StatusBadRequest)
		return
	}
	frame, err := s.cameraManager.GetFrame(s.configuredDevice)
	if err != nil {
		s.addLog("ERROR", fmt.Sprintf("Screenshot error: %v", err))
		http.Error(w, "Failed to capture screenshot", http.StatusInternalServerError)
		return
	}
	// ALWAYS return the image
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(frame)

	// OPTIONAL: Also save it locally
	storagePath := s.appConfig.StoragePath //
	if storagePath != "" {
		// e.g. /home/pi/birdcaptures or something
		nowStr := time.Now().Format("20060102_150405")
		filename := filepath.Join(storagePath, fmt.Sprintf("screenshot_%s.jpg", nowStr))

		err = os.WriteFile(filename, frame, 0644)
		if err != nil {
			s.addLog("ERROR", fmt.Sprintf("Failed to save screenshot to %s: %v", filename, err))
		} else {
			s.addLog("INFO", fmt.Sprintf("Screenshot saved to %s", filename))
		}
	}
}

func (s *Server) Stop() error {
	if !s.isRunning {
		s.addLog("ERROR", "Server stop requested, but server is not running")
		return fmt.Errorf("server is not running")
	}

	s.addLog("INFO", "Stopping server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		s.addLog("ERROR", fmt.Sprintf("Server shutdown error: %v", err))
		return fmt.Errorf("server shutdown error: %v", err)
	}

	s.isRunning = false
	s.addLog("INFO", "Server stopped successfully!")
	return nil
}

func (s *Server) IsRunning() bool {
	return s.isRunning
}

func (s *Server) Port() string {
	return s.port
}

func (s *Server) SetPort(port string) error {
	if s.isRunning {
		return fmt.Errorf("cannot change port while server is running")
	}
	s.port = port
	return nil
}

func (s *Server) handleWebSocketPreview(w http.ResponseWriter, r *http.Request) {
	s.addLog("INFO", fmt.Sprintf("Preview WS connection from %s", r.RemoteAddr))
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.addLog("ERROR", fmt.Sprintf("Error upgrading preview WS: %v", err))
		return
	}

	s.addLog("INFO", fmt.Sprintf("Preview WS established from: %s", r.RemoteAddr))

	s.wsConnectionsMu.Lock()
	s.wsConnections[conn] = true
	s.wsConnectionsMu.Unlock()

	defer func() {
		conn.Close()
		s.wsConnectionsMu.Lock()
		delete(s.wsConnections, conn)
		s.wsConnectionsMu.Unlock()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			s.addLog("ERROR", fmt.Sprintf("Preview WS read error: %v", err))
			break
		}
	}
}

func (s *Server) handleWebSocketLive(w http.ResponseWriter, r *http.Request) {
	s.addLog("INFO", fmt.Sprintf("LiveMonitor WS attempt from %s", r.RemoteAddr))
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.addLog("ERROR", fmt.Sprintf("Error upgrading Live WS: %v", err))
		return
	}

	s.addLog("INFO", fmt.Sprintf("LiveMonitor WS established from: %s", r.RemoteAddr))

	s.wsConnectionsMu.Lock()
	s.wsConnections[conn] = true
	s.wsConnectionsMu.Unlock()

	defer func() {
		conn.Close()
		s.wsConnectionsMu.Lock()
		delete(s.wsConnections, conn)
		s.wsConnectionsMu.Unlock()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			s.addLog("ERROR", fmt.Sprintf("LiveMonitor WS read error: %v", err))
			break
		}
	}
}

func (s *Server) handleWebSocketCamera(w http.ResponseWriter, r *http.Request) {
	s.addLog("INFO", fmt.Sprintf("Websocket connection attempt from: %s", r.RemoteAddr))
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.addLog("ERROR", fmt.Sprintf("Error upgrading websocket connection: %v", err))
		return
	}

	s.addLog("INFO", fmt.Sprintf("Websocket connection established from: %s", r.RemoteAddr))

	s.wsConnectionsMu.Lock()
	s.wsConnections[conn] = true
	s.wsConnectionsMu.Unlock()

	defer func() {
		conn.Close()
		s.wsConnectionsMu.Lock()
		delete(s.wsConnections, conn)
		s.wsConnectionsMu.Unlock()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			s.addLog("ERROR", fmt.Sprintf("Error reading message from websocket: %v", err))
			break
		}
	}
}

func (s *Server) removeConnection(conn *websocket.Conn) {
	s.wsConnectionsMu.Lock()
	defer s.wsConnectionsMu.Unlock()

	// close our actual connection
	_ = conn.Close()

	// Remove the now closed connection from our map
	delete(s.wsConnections, conn)
}

func (s *Server) BroadcastFrame(frameBytes []byte) {
	// Lock our map just long enough to see the current connections
	s.wsConnectionsMu.Lock()
	defer s.wsConnectionsMu.Unlock()
	// For each connection, spin up a goruotine that does he actual WriteMessage
	for conn := range s.wsConnections {
		go func(c *websocket.Conn) {

			if err := c.WriteMessage(websocket.BinaryMessage, frameBytes); err != nil {
				s.addLog("ERROR", fmt.Sprintf("Error writing message to websocket: %v", err))
				s.removeConnection(c)
			}
		}(conn)
	}
}

func (s *Server) addLog(level, message string) {
	logEntry := LogEntry{
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("[%s] %s", level, message),
	}

	s.logMutex.Lock()
	s.logBuffer = append(s.logBuffer, logEntry)
	if len(s.logBuffer) > 100 {
		s.logBuffer = s.logBuffer[1:]
	}
	s.logMutex.Unlock()

	// Debugging: Confirm callback is invoked
	if s.logCallback != nil {
		s.logCallback(level, message)
	} else {
		fmt.Println("Warning: logCallback is nil")
	}
}
