// internal/server/server.go
package server

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type LogEntry struct {
	Timestamp time.Time
	Message   string
}

type logWriter struct {
	server *Server
}

type Server struct {
	server          *http.Server
	port            string
	isRunning       bool
	logger          *log.Logger
	logBuffer       []LogEntry
	logMutex        sync.RWMutex
	upgrader        websocket.Upgrader
	wsConnections   map[*websocket.Conn]bool
	wsConnectionsMu sync.RWMutex
}

func New(port string) *Server {
	s := &Server{
		port:      port,
		logBuffer: make([]LogEntry, 0, 100),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		wsConnections: make(map[*websocket.Conn]bool),
	}
	s.logger = log.New(&logWriter{s}, "[WEBSERVER] ", log.LstdFlags)
	return s
}

func (s *Server) Start() error {
	if s.isRunning {
		s.logger.Printf("Server is already running on port %s", s.port)
		return fmt.Errorf("server is already running!")
	}

	mux := http.NewServeMux()
	// Websocket route
	mux.HandleFunc("/ws/camera", s.handleWebSocketCamera)

	// Add static file serving
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Add logging middleware
	loggingMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		s.logger.Printf("Started %s %s", r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
		s.logger.Printf("Completed %s in %v", r.URL.Path, time.Since(start))
	})

	// Basic routes
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
			s.logger.Printf("Error parsing template: %v", err)
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	mux.HandleFunc("/live-monitor", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("web/templates/live-monitor.html")
		if err != nil {
			s.logger.Printf("Error parsing template: %v", err)
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	s.server = &http.Server{
		Addr:    ":" + s.port,
		Handler: loggingMux,
	}

	// Start server in a goroutine
	go func() {
		s.logger.Printf("Starting server on port %s", s.port)
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Printf("HTTP server error: %v", err)
		}
	}()

	s.isRunning = true
	s.logger.Printf("Server is running on port %s", s.port)
	return nil
}

func (s *Server) GetRecentLogs(count int) []LogEntry {
	s.logMutex.RLock()
	defer s.logMutex.RUnlock()
	if len(s.logBuffer) <= count {
		return s.logBuffer
	}
	return s.logBuffer[len(s.logBuffer)-count:]
}

func (s *Server) Stop() error {
	if !s.isRunning {
		s.logger.Print("Server stop requested, but server is not running")
		return fmt.Errorf("server is not running")
	}

	s.logger.Print("Stopping server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Printf("Server shutdown error: %v", err)
		return fmt.Errorf("server shutdown error: %v", err)
	}

	s.isRunning = false
	s.logger.Print("Server stopped successfully!")
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
	s.logger.Print("Websocket connection attempt from: %s", r.RemoteAddr)
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Printf("Error upgrading websocket connection: %v", err)
		return
	}

	s.logger.Printf("Websocket connection established from: %s", r.RemoteAddr)

	s.wsConnectionsMu.Lock()
	s.wsConnections[conn] = true
	s.wsConnectionsMu.Unlock()

	defer func() {
		conn.Close()
		s.wsConnectionsMu.Lock()
		delete(s.wsConnections, conn)
		s.wsConnectionsMu.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			s.logger.Printf("Error reading message from websocket: %v", err)
			break
		}
	}
}

func (s *Server) BroadcastFrame(frameBytes []byte) {
	s.wsConnectionsMu.Lock()
	defer s.wsConnectionsMu.Unlock()
	for conn := range s.wsConnections {
		err := conn.WriteMessage(websocket.BinaryMessage, frameBytes)
		if err != nil {
			s.logger.Printf("Error writing message to websocket: %v", err)
			conn.Close()
			delete(s.wsConnections, conn)
		}
	}
}

// Logging middleware support functions
func (w *logWriter) Write(p []byte) (n int, err error) {
	w.server.logMutex.Lock()
	w.server.logBuffer = append(w.server.logBuffer, LogEntry{
		Timestamp: time.Now(),
		Message:   string(p),
	})
	// Keep only last 100 logs
	if len(w.server.logBuffer) > 100 {
		w.server.logBuffer = w.server.logBuffer[1:]
	}
	w.server.logMutex.Unlock()
	return len(p), nil
}
