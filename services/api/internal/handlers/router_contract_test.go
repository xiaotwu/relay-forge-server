package handlers

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/relay-forge/relay-forge/services/api/internal/config"
)

func TestRouterCriticalPathsMatchOpenAPI(t *testing.T) {
	cfg := &config.Config{
		CORS: config.CORSConfig{Origins: []string{"http://localhost:3000"}},
		Auth: config.AuthConfig{
			JWTSecret:  "test-secret",
			AccessTTL:  time.Minute,
			RefreshTTL: time.Hour,
		},
	}

	router, ok := NewRouter(cfg, nil).(chi.Routes)
	if !ok {
		t.Fatal("router does not expose chi routes")
	}

	routes := make(map[string]struct{})
	if err := chi.Walk(router, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes[method+" "+route] = struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	specBytes, err := os.ReadFile("../../../../openapi/relayforge.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI spec: %v", err)
	}
	spec := string(specBytes)

	critical := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/guilds/{guildID}/channels/{channelID}"},
		{http.MethodPatch, "/api/v1/guilds/{guildID}/channels/{channelID}"},
		{http.MethodDelete, "/api/v1/guilds/{guildID}/channels/{channelID}"},
		{http.MethodPost, "/api/v1/guilds/{guildID}/roles/{roleID}/members/{userID}"},
		{http.MethodDelete, "/api/v1/guilds/{guildID}/roles/{roleID}/members/{userID}"},
		{http.MethodPost, "/api/v1/channels/{channelID}/messages/{messageID}/reactions"},
		{http.MethodPut, "/api/v1/channels/{channelID}/messages/{messageID}/reactions/{emoji}"},
		{http.MethodGet, "/api/v1/admin/dashboard/stats"},
		{http.MethodGet, "/api/v1/admin/audit"},
		{http.MethodGet, "/api/v1/admin/reports"},
		{http.MethodPost, "/api/v1/admin/users/{userID}/enable"},
		{http.MethodPost, "/api/v1/admin/users/{userID}/disable"},
	}

	for _, item := range critical {
		if _, ok := routes[item.method+" "+item.path]; !ok {
			t.Fatalf("router missing %s %s", item.method, item.path)
		}
		if !strings.Contains(spec, item.path+":") {
			t.Fatalf("OpenAPI spec missing %s", item.path)
		}
	}
}
