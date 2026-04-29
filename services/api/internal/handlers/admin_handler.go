package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/relay-forge/relay-forge/services/api/internal/errors"
	"github.com/relay-forge/relay-forge/services/api/internal/middleware"
)

type AdminHandler struct {
	users    adminUserStore
	guilds   adminGuildStore
	sessions adminSessionStore
	pool     *pgxpool.Pool
}

type adminUserStore interface {
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type adminGuildStore interface {
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type adminSessionStore interface {
	DeleteAllForUser(ctx context.Context, userID uuid.UUID) error
	RevokeAllRefreshTokensForUser(ctx context.Context, userID uuid.UUID) error
}

func NewAdminHandler(
	users adminUserStore,
	guilds adminGuildStore,
	sessions adminSessionStore,
	pool *pgxpool.Pool,
) *AdminHandler {
	return &AdminHandler{users: users, guilds: guilds, sessions: sessions, pool: pool}
}

func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles := middleware.GetUserRoles(r.Context())
		for _, role := range roles {
			if role == "admin" {
				next.ServeHTTP(w, r)
				return
			}
		}
		respondError(w, forbiddenErr("admin access required"))
	})
}

type adminUserRow struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type adminGuildRow struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	OwnerUsername string    `json:"owner_username"`
	MemberCount   int       `json:"member_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type auditRow struct {
	ID        uuid.UUID `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Timestamp time.Time `json:"timestamp"`
	Details   *string   `json:"details,omitempty"`
}

type reportRow struct {
	ID        uuid.UUID `json:"id"`
	Reporter  string    `json:"reporter"`
	Target    string    `json:"target"`
	Reason    string    `json:"reason"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func paginationMeta(total, limit, offset int) map[string]int {
	page := (offset / limit) + 1
	totalPages := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}
	return map[string]int{
		"total":       total,
		"limit":       limit,
		"offset":      offset,
		"page":        page,
		"page_size":   limit,
		"total_pages": totalPages,
	}
}

func (h *AdminHandler) DashboardStats(w http.ResponseWriter, r *http.Request) {
	stats := struct {
		TotalUsers     int `json:"total_users"`
		TotalGuilds    int `json:"total_guilds"`
		ActiveSessions int `json:"active_sessions"`
		StorageUsageMB int `json:"storage_usage_mb"`
	}{}

	err := h.pool.QueryRow(r.Context(), `
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM guilds),
			(SELECT COUNT(*) FROM sessions WHERE expires_at > NOW()),
			COALESCE((SELECT CEIL(SUM(file_size)::numeric / 1048576) FROM file_uploads), 0)::int
	`).Scan(&stats.TotalUsers, &stats.TotalGuilds, &stats.ActiveSessions, &stats.StorageUsageMB)
	if err != nil {
		respondError(w, apperrors.Internal("failed to load dashboard stats"))
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

func (h *AdminHandler) RecentActivity(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `
		SELECT a.id, COALESCE(u.username, 'system'), a.action::text,
		       COALESCE(a.target_type || ':' || a.target_id::text, 'platform'),
		       a.created_at, COALESCE(a.reason, '')
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.actor_id
		ORDER BY a.created_at DESC
		LIMIT 20`)
	if err != nil {
		respondError(w, apperrors.Internal("failed to load recent activity"))
		return
	}
	defer rows.Close()

	activity := make([]auditRow, 0)
	for rows.Next() {
		var row auditRow
		var details string
		if err := rows.Scan(&row.ID, &row.Actor, &row.Action, &row.Target, &row.Timestamp, &details); err != nil {
			respondError(w, apperrors.Internal("failed to scan recent activity"))
			return
		}
		if details != "" {
			row.Details = &details
		}
		activity = append(activity, row)
	}
	if err := rows.Err(); err != nil {
		respondError(w, apperrors.Internal("failed to iterate recent activity"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"activities": activity})
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	where := ""
	countArgs := []any{}
	args := []any{limit, offset}
	if search != "" {
		where = `WHERE username ILIKE '%' || $3 || '%' OR email ILIKE '%' || $3 || '%' OR COALESCE(display_name, '') ILIKE '%' || $3 || '%'`
		args = append(args, search)
		countArgs = append(countArgs, search)
	}

	var total int
	countWhere := ""
	if search != "" {
		countWhere = `WHERE username ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%' OR COALESCE(display_name, '') ILIKE '%' || $1 || '%'`
	}
	countQuery := `SELECT COUNT(*) FROM users ` + countWhere
	if err := h.pool.QueryRow(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		respondError(w, apperrors.Internal("failed to count users"))
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, username, email,
		       CASE WHEN is_disabled THEN 'disabled' ELSE 'active' END AS status,
		       created_at
		FROM users
		`+where+`
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, args...)
	if err != nil {
		respondError(w, apperrors.Internal("failed to list users"))
		return
	}
	defer rows.Close()

	users := make([]adminUserRow, 0)
	for rows.Next() {
		var row adminUserRow
		if err := rows.Scan(&row.ID, &row.Username, &row.Email, &row.Status, &row.CreatedAt); err != nil {
			respondError(w, apperrors.Internal("failed to scan users"))
			return
		}
		users = append(users, row)
	}
	if err := rows.Err(); err != nil {
		respondError(w, apperrors.Internal("failed to iterate users"))
		return
	}

	respondJSONWithMeta(w, http.StatusOK, users, paginationMeta(total, limit, offset))
}

func (h *AdminHandler) ListGuilds(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	where := ""
	countArgs := []any{}
	args := []any{limit, offset}
	if search != "" {
		where = `WHERE g.name ILIKE '%' || $3 || '%' OR COALESCE(owner.username, '') ILIKE '%' || $3 || '%'`
		args = append(args, search)
		countArgs = append(countArgs, search)
	}

	var total int
	countWhere := ""
	if search != "" {
		countWhere = `WHERE g.name ILIKE '%' || $1 || '%' OR COALESCE(owner.username, '') ILIKE '%' || $1 || '%'`
	}
	countQuery := `SELECT COUNT(*) FROM guilds g LEFT JOIN users owner ON owner.id = g.owner_id ` + countWhere
	if err := h.pool.QueryRow(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		respondError(w, apperrors.Internal("failed to count guilds"))
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT g.id, g.name, COALESCE(owner.username, 'unknown'), g.member_count, g.created_at
		FROM guilds g
		LEFT JOIN users owner ON owner.id = g.owner_id
		`+where+`
		ORDER BY g.created_at DESC
		LIMIT $1 OFFSET $2`, args...)
	if err != nil {
		respondError(w, apperrors.Internal("failed to list guilds"))
		return
	}
	defer rows.Close()

	guilds := make([]adminGuildRow, 0)
	for rows.Next() {
		var row adminGuildRow
		if err := rows.Scan(&row.ID, &row.Name, &row.OwnerUsername, &row.MemberCount, &row.CreatedAt); err != nil {
			respondError(w, apperrors.Internal("failed to scan guilds"))
			return
		}
		guilds = append(guilds, row)
	}
	if err := rows.Err(); err != nil {
		respondError(w, apperrors.Internal("failed to iterate guilds"))
		return
	}

	respondJSONWithMeta(w, http.StatusOK, guilds, paginationMeta(total, limit, offset))
}

func (h *AdminHandler) DisableUser(w http.ResponseWriter, r *http.Request) {
	currentUserID := middleware.GetUserID(r.Context())
	userID, err := parseUUID(chi.URLParam(r, "userID"))
	if err != nil {
		respondError(w, err)
		return
	}
	if userID == currentUserID {
		respondError(w, apperrors.Forbidden("cannot disable your own admin account"))
		return
	}

	if err := h.users.SoftDelete(r.Context(), userID); err != nil {
		respondError(w, err)
		return
	}
	if err := h.sessions.DeleteAllForUser(r.Context(), userID); err != nil {
		respondError(w, err)
		return
	}
	if err := h.sessions.RevokeAllRefreshTokensForUser(r.Context(), userID); err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (h *AdminHandler) EnableUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUUID(chi.URLParam(r, "userID"))
	if err != nil {
		respondError(w, err)
		return
	}

	result, err := h.pool.Exec(r.Context(), `
		UPDATE users SET is_disabled = false, updated_at = NOW() WHERE id = $1`,
		userID,
	)
	if err != nil {
		respondError(w, apperrors.Internal("failed to enable user"))
		return
	}
	if result.RowsAffected() == 0 {
		respondError(w, apperrors.NotFound("user not found"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

func (h *AdminHandler) DeleteGuild(w http.ResponseWriter, r *http.Request) {
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.guilds.SoftDelete(r.Context(), guildID); err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	where := ""
	countArgs := []any{}
	args := []any{limit, offset}
	if action != "" && action != "All" {
		where = `WHERE a.action::text = $3`
		args = append(args, action)
		countArgs = append(countArgs, action)
	}

	var total int
	countWhere := ""
	if action != "" && action != "All" {
		countWhere = `WHERE a.action::text = $1`
	}
	if err := h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM audit_logs a `+countWhere, countArgs...).Scan(&total); err != nil {
		respondError(w, apperrors.Internal("failed to count audit logs"))
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT a.id, COALESCE(u.username, 'system'), a.action::text,
		       COALESCE(a.target_type || ':' || a.target_id::text, 'platform'),
		       a.created_at, COALESCE(a.reason, '')
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.actor_id
		`+where+`
		ORDER BY a.created_at DESC
		LIMIT $1 OFFSET $2`, args...)
	if err != nil {
		respondError(w, apperrors.Internal("failed to list audit logs"))
		return
	}
	defer rows.Close()

	logs := make([]auditRow, 0)
	for rows.Next() {
		var row auditRow
		var details string
		if err := rows.Scan(&row.ID, &row.Actor, &row.Action, &row.Target, &row.Timestamp, &details); err != nil {
			respondError(w, apperrors.Internal("failed to scan audit logs"))
			return
		}
		if details != "" {
			row.Details = &details
		}
		logs = append(logs, row)
	}
	if err := rows.Err(); err != nil {
		respondError(w, apperrors.Internal("failed to iterate audit logs"))
		return
	}

	respondJSONWithMeta(w, http.StatusOK, logs, paginationMeta(total, limit, offset))
}

func (h *AdminHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	var total int
	if err := h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM reports`).Scan(&total); err != nil {
		respondError(w, apperrors.Internal("failed to count reports"))
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT r.id, COALESCE(reporter.username, 'unknown'), r.target_type || ':' || r.target_id::text,
		       r.reason, r.status::text, r.created_at
		FROM reports r
		INNER JOIN users reporter ON reporter.id = r.reporter_id
		ORDER BY r.created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		respondError(w, apperrors.Internal("failed to list reports"))
		return
	}
	defer rows.Close()

	reports := make([]reportRow, 0)
	for rows.Next() {
		var row reportRow
		if err := rows.Scan(&row.ID, &row.Reporter, &row.Target, &row.Reason, &row.Status, &row.CreatedAt); err != nil {
			respondError(w, apperrors.Internal("failed to scan reports"))
			return
		}
		reports = append(reports, row)
	}
	if err := rows.Err(); err != nil {
		respondError(w, apperrors.Internal("failed to iterate reports"))
		return
	}

	respondJSONWithMeta(w, http.StatusOK, reports, paginationMeta(total, limit, offset))
}

func (h *AdminHandler) ResolveReport(w http.ResponseWriter, r *http.Request) {
	h.updateReportStatus(w, r, "resolved")
}

func (h *AdminHandler) DismissReport(w http.ResponseWriter, r *http.Request) {
	h.updateReportStatus(w, r, "dismissed")
}

func (h *AdminHandler) updateReportStatus(w http.ResponseWriter, r *http.Request, status string) {
	reportID, err := parseUUID(chi.URLParam(r, "reportID"))
	if err != nil {
		respondError(w, err)
		return
	}

	result, err := h.pool.Exec(r.Context(), `
		UPDATE reports
		SET status = $2::report_status, moderator_id = $3, resolved_at = NOW()
		WHERE id = $1`,
		reportID, status, middleware.GetUserID(r.Context()),
	)
	if err != nil {
		respondError(w, apperrors.Internal("failed to update report"))
		return
	}
	if result.RowsAffected() == 0 {
		respondError(w, apperrors.NotFound("report not found"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (h *AdminHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings := map[string]any{
		"registration_enabled":           true,
		"email_verification_required":    false,
		"max_upload_size_mb":             25,
		"rate_limit_requests_per_minute": 60,
		"rate_limit_burst_size":          10,
		"maintenance_mode":               false,
	}

	var raw []byte
	err := h.pool.QueryRow(r.Context(), `SELECT value FROM system_settings WHERE key = 'platform'`).Scan(&raw)
	if err == nil {
		_ = json.Unmarshal(raw, &settings)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		respondError(w, apperrors.Internal("failed to load settings"))
		return
	}

	respondJSON(w, http.StatusOK, settings)
}

func (h *AdminHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings map[string]any
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		respondError(w, apperrors.Validation("invalid settings payload", nil))
		return
	}

	raw, err := json.Marshal(settings)
	if err != nil {
		respondError(w, apperrors.Validation("invalid settings payload", nil))
		return
	}

	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO system_settings (key, value, updated_by, updated_at)
		VALUES ('platform', $1, $2, NOW())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		raw, middleware.GetUserID(r.Context()),
	)
	if err != nil {
		respondError(w, apperrors.Internal("failed to save settings"))
		return
	}

	respondJSON(w, http.StatusOK, settings)
}
