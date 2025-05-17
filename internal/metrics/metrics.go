package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

var birdsSeen uint64
var birdsCaptured uint64
var httpRequests uint64
var startTime time.Time

// SetStartTime records when the application began running so uptime can be
// calculated.
func SetStartTime(t time.Time) {
	startTime = t
}

func IncrementBirdsSeen() {
	atomic.AddUint64(&birdsSeen, 1)
}

func IncrementBirdsCaptured() {
	atomic.AddUint64(&birdsCaptured, 1)
}

// IncrementRequests increases the http request counter.
func IncrementRequests() {
	atomic.AddUint64(&httpRequests, 1)
}

// RequestMiddleware wraps an http.Handler and increments the request counter
// for each request before calling the next handler.
func RequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		IncrementRequests()
		next.ServeHTTP(w, r)
	})
}

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP birds_seen Number of birds detected\n")
	fmt.Fprintf(w, "# TYPE birds_seen counter\n")
	fmt.Fprintf(w, "birds_seen %d\n", atomic.LoadUint64(&birdsSeen))
	fmt.Fprintf(w, "# HELP birds_captured Number of birds captured\n")
	fmt.Fprintf(w, "# TYPE birds_captured counter\n")
	fmt.Fprintf(w, "birds_captured %d\n", atomic.LoadUint64(&birdsCaptured))
	fmt.Fprintf(w, "# HELP http_requests_total Total HTTP requests\n")
	fmt.Fprintf(w, "# TYPE http_requests_total counter\n")
	fmt.Fprintf(w, "http_requests_total %d\n", atomic.LoadUint64(&httpRequests))
	if !startTime.IsZero() {
		uptime := time.Since(startTime).Seconds()
		fmt.Fprintf(w, "# HELP uptime_seconds Application uptime in seconds\n")
		fmt.Fprintf(w, "# TYPE uptime_seconds gauge\n")
		fmt.Fprintf(w, "uptime_seconds %.0f\n", uptime)
	}
}

func BirdsSeenHandler(w http.ResponseWriter, r *http.Request) {
	IncrementBirdsSeen()
	w.WriteHeader(http.StatusNoContent)
}

func BirdsCapturedHandler(w http.ResponseWriter, r *http.Request) {
	IncrementBirdsCaptured()
	w.WriteHeader(http.StatusNoContent)
}
