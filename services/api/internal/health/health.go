package health

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type status struct {
	Status   string            `json:"status"`
	Checks   map[string]string `json:"checks,omitempty"`
}

func Healthz(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(status{Status: "ok"})
	}
}

func Readyz(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := make(map[string]string)

		if err := db.Ping(r.Context()); err != nil {
			checks["database"] = "fail: " + err.Error()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(status{Status: "unavailable", Checks: checks})
			return
		}
		checks["database"] = "ok"

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(status{Status: "ready", Checks: checks})
	}
}
