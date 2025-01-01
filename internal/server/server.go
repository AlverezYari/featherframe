// internal/server/server.go
package server

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type LogEntry struct {
	Timestamp time.Time
	Message   string
}

type Server struct {
	server          *http.Server
	port            string
	isRunning       bool
	logBuffer       []LogEntry
	logMutex        sync.RWMutex
	upgrader        websocket.Upgrader
	wsConnections   map[*websocket.Conn]bool
	wsConnectionsMu sync.RWMutex
	logCallback     func(level, message string) // Callback for forwarding logs
}

func New(port string, logCallback func(level, message string)) *Server {
	return &Server{
		port:        port,
		logBuffer:   make([]LogEntry, 0, 100),
		logCallback: logCallback,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		wsConnections: make(map[*websocket.Conn]bool),
	}
}

func (s *Server) Start() error {
	if s.isRunning {
		s.addLog("ERROR", fmt.Sprintf("Server is already running on port %s", s.port))
		return fmt.Errorf("server is already running")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/camera", s.handleWebSocketCamera)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "FeatherFrame Web Interface")
	})

	mux.HandleFunc("/focus", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Camera Focus Interface")
	})

	mux.HandleFunc("/monitor", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Bird Monitoring Interface")
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

	s.server = &http.Server{
		Addr:    ":" + s.port,
		Handler: mux,
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

func (s *Server) BroadcastFrame(frameBytes []byte) {
	s.wsConnectionsMu.Lock()
	defer s.wsConnectionsMu.Unlock()
	for conn := range s.wsConnections {
		if err := conn.WriteMessage(websocket.BinaryMessage, frameBytes); err != nil {
			s.addLog("ERROR", fmt.Sprintf("Error writing message to websocket: %v", err))
			conn.Close()
			delete(s.wsConnections, conn)
		}
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
	fmt.Printf("Server Log: [%s] %s\n", level, message)
	if s.logCallback != nil {
		s.logCallback(level, message)
	} else {
		fmt.Println("Warning: logCallback is nil")
	}
}
