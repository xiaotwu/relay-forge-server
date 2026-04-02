package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

// RateLimit returns a chi middleware that limits each client to rps requests
// per second with the given burst capacity. Clients are identified by their
// real IP address. Requests exceeding the limit receive a 429 Too Many
// Requests response.
func RateLimit(rps int, burst int) func(http.Handler) http.Handler {
	// httprate works with a window and a request limit within that window.
	// To approximate "rps requests per second with burst", we use a 1-second
	// window with a limit of rps+burst to allow short bursts, then rely on
	// the sliding window to enforce the sustained rate.
	limit := rps + burst
	window := time.Second

	return httprate.Limit(
		limit,
		window,
		httprate.WithKeyFuncs(httprate.KeyByRealIP),
		httprate.WithLimitHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":"rate_limited","message":"too many requests, please try again later"}`))
		})),
	)
}
