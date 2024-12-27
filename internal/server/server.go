// internal/server/server.go
package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type LogEntry struct {
	Timestamp time.Time
	Message   string
}

type logWriter struct {
	server *Server
}

type Server struct {
	server    *http.Server
	port      string
	isRunning bool
	logger    *log.Logger
	logBuffer []LogEntry
	logMutex  sync.RWMutex
}

func New(port string) *Server {
	s := &Server{
		port:      port,
		logBuffer: make([]LogEntry, 0, 100),
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
