package server

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// logging records the method, path, status, and duration of each request.
// The log line is written from a defer so it still fires if a handler
// panics and recoverPanic (registered around this middleware) recovers it.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
		}()
		next.ServeHTTP(rec, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// recoverPanic recovers a panicking handler, logs it, and writes a generic
// 500 so a bad request can't take a goroutine down or spill request
// contents into a stack trace shown to the client. It must wrap logging
// (registered before it in routes()) so the access log line still fires.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered: %v\n%s", rec, debug.Stack())
				respondError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
