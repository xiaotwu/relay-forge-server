package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const requestIDHeader = "X-Request-ID"

// RequestID returns a chi middleware that generates a unique request ID for
// each incoming request. If the client provides an X-Request-ID header, it is
// reused; otherwise a new UUID is generated. The ID is set on the response
// header and stored in the request context.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(requestIDHeader)
			if rid == "" {
				rid = uuid.New().String()
			}

			w.Header().Set(requestIDHeader, rid)

			ctx := r.Context()
			ctx = zerolog.Ctx(ctx).With().Str("request_id", rid).Logger().WithContext(ctx)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter, allowing chi's middleware
// (and http.ResponseController) to access the original writer's interfaces.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// RequestLogger returns a chi middleware that logs each request with method,
// path, status code, duration, and request ID using zerolog.
func RequestLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := newResponseWriter(w)

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			logger := log.Ctx(r.Context())

			var evt *zerolog.Event
			switch {
			case wrapped.statusCode >= 500:
				evt = logger.Error()
			case wrapped.statusCode >= 400:
				evt = logger.Warn()
			default:
				evt = logger.Info()
			}

			evt.
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapped.statusCode).
				Dur("duration", duration).
				Str("remote_addr", r.RemoteAddr).
				Str("request_id", w.Header().Get(requestIDHeader)).
				Msg("request completed")
		})
	}
}
