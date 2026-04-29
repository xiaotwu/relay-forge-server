package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/relay-forge/relay-forge/services/realtime/internal/config"
)

const (
	writeWait       = 10 * time.Second
	pongWait        = 60 * time.Second
	pingPeriod      = (pongWait * 9) / 10
	authCheckPeriod = 30 * time.Second
	maxMessageSize  = 4096
)

const EventsChannel = "relayforge.events"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Event types sent over WebSocket
const (
	EventMessage       = "MESSAGE_CREATE"
	EventMessageUpdate = "MESSAGE_UPDATE"
	EventMessageDelete = "MESSAGE_DELETE"
	EventTypingStart   = "TYPING_START"
	EventTypingStop    = "TYPING_STOP"
	EventPresence      = "PRESENCE_UPDATE"
	EventReadState     = "READ_STATE_UPDATE"
	EventGuildUpdate   = "GUILD_UPDATE"
	EventChannelUpdate = "CHANNEL_UPDATE"
	EventMemberJoin    = "GUILD_MEMBER_ADD"
	EventMemberLeave   = "GUILD_MEMBER_REMOVE"
	EventHeartbeat     = "HEARTBEAT"
	EventHeartbeatAck  = "HEARTBEAT_ACK"
)

type Event struct {
	Type      string          `json:"type"`
	ChannelID string          `json:"channel_id,omitempty"`
	GuildID   string          `json:"guild_id,omitempty"`
	UserID    string          `json:"user_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type pubsubEvent struct {
	Type    string          `json:"type"`
	GuildID *uuid.UUID      `json:"guild_id,omitempty"`
	UserIDs []uuid.UUID     `json:"user_ids,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type outboundEnvelope struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data,omitempty"`
	Seq       int64           `json:"seq"`
	Timestamp string          `json:"timestamp"`
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	userID   uuid.UUID
	guildIDs map[uuid.UUID]bool
	send     chan []byte
	done     chan struct{}
}

type Hub struct {
	cfg        *config.Config
	db         *pgxpool.Pool
	redis      *redis.Client
	clients    map[uuid.UUID]map[*Client]bool // userID -> set of clients
	guilds     map[uuid.UUID]map[*Client]bool // guildID -> set of clients
	register   chan *Client
	unregister chan *Client
	broadcast  chan *guildMessage
	userActive func(context.Context, uuid.UUID) (bool, error)
	mu         sync.RWMutex
	seq        atomic.Int64
	quit       chan struct{}
}

type guildMessage struct {
	guildID uuid.UUID
	data    []byte
}

type typingPayload struct {
	ChannelID string `json:"channelId"`
	GuildID   string `json:"guildId,omitempty"`
	UserID    string `json:"userId,omitempty"`
	Username  string `json:"username,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

func New(cfg *config.Config, db *pgxpool.Pool) *Hub {
	h := &Hub{
		cfg: cfg,
		db:  db,
		redis: redis.NewClient(&redis.Options{
			Addr:     cfg.Valkey.Addr(),
			Password: cfg.Valkey.Password,
			DB:       cfg.Valkey.DB,
		}),
		clients:    make(map[uuid.UUID]map[*Client]bool),
		guilds:     make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
		broadcast:  make(chan *guildMessage, 4096),
		quit:       make(chan struct{}),
	}
	h.userActive = h.isUserActive
	return h
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[client.userID]; !ok {
				h.clients[client.userID] = make(map[*Client]bool)
			}
			h.clients[client.userID][client] = true
			for gID := range client.guildIDs {
				if _, ok := h.guilds[gID]; !ok {
					h.guilds[gID] = make(map[*Client]bool)
				}
				h.guilds[gID][client] = true
			}
			h.mu.Unlock()
			log.Info().Str("user_id", client.userID.String()).Msg("client connected")

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.userID]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.clients, client.userID)
				}
			}
			for gID := range client.guildIDs {
				if clients, ok := h.guilds[gID]; ok {
					delete(clients, client)
					if len(clients) == 0 {
						delete(h.guilds, gID)
					}
				}
			}
			h.mu.Unlock()
			close(client.send)
			log.Info().Str("user_id", client.userID.String()).Msg("client disconnected")

		case msg := <-h.broadcast:
			h.mu.RLock()
			clients := h.guilds[msg.guildID]
			for client := range clients {
				select {
				case client.send <- msg.data:
				default:
					go func(c *Client) { h.unregister <- c }(client)
				}
			}
			h.mu.RUnlock()

		case <-h.quit:
			return
		}
	}
}

func (h *Hub) Shutdown() {
	close(h.quit)
	if h.redis != nil {
		if err := h.redis.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close valkey client")
		}
	}
}

func (h *Hub) BroadcastToGuild(guildID uuid.UUID, event Event) {
	data, err := h.buildEnvelope(event.Type, event.Data)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal broadcast event")
		return
	}
	h.broadcast <- &guildMessage{guildID: guildID, data: data}
}

func (h *Hub) SendToUser(userID uuid.UUID, event Event) {
	data, err := h.buildEnvelope(event.Type, event.Data)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal user event")
		return
	}
	h.mu.RLock()
	clients := h.clients[userID]
	for client := range clients {
		select {
		case client.send <- data:
		default:
		}
	}
	h.mu.RUnlock()
}

func (h *Hub) Subscribe(ctx context.Context) {
	if h.redis == nil {
		return
	}

	pubsub := h.redis.Subscribe(ctx, EventsChannel)
	defer func() {
		if err := pubsub.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close valkey pubsub")
		}
	}()

	if _, err := pubsub.Receive(ctx); err != nil {
		log.Warn().Err(err).Msg("failed to subscribe to realtime events")
		return
	}

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			h.handlePubSubMessage(msg.Payload)
		}
	}
}

func (h *Hub) handlePubSubMessage(payload string) {
	var event pubsubEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Warn().Err(err).Msg("failed to decode realtime event")
		return
	}
	event.Type = normalizeEventType(event.Type)
	if event.Type == "" {
		return
	}

	wsEvent := Event{Type: event.Type, Data: event.Data}
	if event.GuildID != nil {
		h.BroadcastToGuild(*event.GuildID, wsEvent)
	}
	for _, userID := range event.UserIDs {
		h.SendToUser(userID, wsEvent)
	}
}

func (h *Hub) buildEnvelope(eventType string, data json.RawMessage) ([]byte, error) {
	return json.Marshal(outboundEnvelope{
		Type:      normalizeEventType(eventType),
		Data:      data,
		Seq:       h.seq.Add(1),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" && !h.isOriginAllowed(origin) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	userID, err := h.validateToken(r.Context(), tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("websocket upgrade failed")
		return
	}

	// Parse guild IDs from query parameter
	guildIDs := make(map[uuid.UUID]bool)
	for _, gidStr := range r.URL.Query()["guilds"] {
		gid, err := uuid.Parse(gidStr)
		if err != nil {
			continue
		}
		isMember, err := h.isGuildMember(r.Context(), gid, userID)
		if err != nil {
			log.Warn().Err(err).Str("guild_id", gid.String()).Str("user_id", userID.String()).Msg("failed to validate websocket guild subscription")
			continue
		}
		if isMember {
			guildIDs[gid] = true
		}
	}

	client := &Client{
		hub:      h,
		conn:     conn,
		userID:   userID,
		guildIDs: guildIDs,
		send:     make(chan []byte, 256),
		done:     make(chan struct{}),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (h *Hub) validateToken(ctx context.Context, tokenStr string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.cfg.JWTSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, jwt.ErrSignatureInvalid
	}

	sub, err := claims.GetSubject()
	if err != nil {
		return uuid.Nil, err
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, err
	}
	active, err := h.isClientActive(ctx, userID)
	if err != nil {
		return uuid.Nil, err
	}
	if !active {
		return uuid.Nil, fmt.Errorf("user disabled or not found")
	}

	return userID, nil
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		if err := c.conn.Close(); err != nil {
			log.Warn().Err(err).Str("user_id", c.userID.String()).Msg("failed to close websocket connection")
		}
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Warn().Err(err).Str("user_id", c.userID.String()).Msg("failed to set read deadline")
		return
	}
	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			log.Warn().Err(err).Str("user_id", c.userID.String()).Msg("failed to refresh read deadline")
			return err
		}
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Warn().Err(err).Str("user_id", c.userID.String()).Msg("unexpected close")
			}
			return
		}

		var event Event
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}

		event.UserID = c.userID.String()
		c.handleEvent(event)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	authTicker := time.NewTicker(authCheckPeriod)
	defer func() {
		ticker.Stop()
		authTicker.Stop()
		if err := c.conn.Close(); err != nil {
			log.Warn().Err(err).Str("user_id", c.userID.String()).Msg("failed to close websocket connection")
		}
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Warn().Err(err).Str("user_id", c.userID.String()).Msg("failed to set write deadline")
				return
			}
			if !ok {
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Warn().Err(err).Str("user_id", c.userID.String()).Msg("failed to write close message")
				}
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Warn().Err(err).Str("user_id", c.userID.String()).Msg("failed to set ping write deadline")
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-authTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			active, err := c.hub.isClientActive(ctx, c.userID)
			cancel()
			if err != nil || !active {
				log.Info().Str("user_id", c.userID.String()).Err(err).Msg("closing websocket for inactive user")
				return
			}
		}
	}
}

func (c *Client) handleEvent(event Event) {
	switch normalizeEventType(event.Type) {
	case EventTypingStart:
		outbound, ok := buildTypingBroadcast(event)
		if !ok {
			return
		}
		if gid, err := uuid.Parse(outbound.GuildID); err == nil {
			c.hub.BroadcastToGuild(gid, outbound)
		}
	case EventReadState:
		// Read state updates are per-user, echoed back
		c.hub.SendToUser(c.userID, event)
	case EventPresence:
		for gid := range c.guildIDs {
			c.hub.BroadcastToGuild(gid, event)
		}
	case EventHeartbeat:
		c.hub.SendToUser(c.userID, Event{
			Type: EventHeartbeatAck,
			Data: event.Data,
		})
	}
}

func normalizeEventType(eventType string) string {
	switch strings.ToUpper(eventType) {
	case "MESSAGE.CREATE", "MESSAGE_CREATE":
		return EventMessage
	case "MESSAGE.UPDATE", "MESSAGE_UPDATE":
		return EventMessageUpdate
	case "MESSAGE.DELETE", "MESSAGE_DELETE":
		return EventMessageDelete
	case "TYPING_START":
		return EventTypingStart
	case "TYPING_STOP":
		return EventTypingStop
	case "TYPING.START":
		return EventTypingStart
	case "TYPING.STOP":
		return EventTypingStop
	case "READ_STATE.UPDATE", "READ_STATE_UPDATE":
		return EventReadState
	case "PRESENCE_UPDATE":
		return EventPresence
	case "PRESENCE.UPDATE":
		return EventPresence
	case "HEARTBEAT":
		return EventHeartbeat
	case "HEARTBEAT_ACK":
		return EventHeartbeatAck
	case "DM.MESSAGE.CREATE", "DM_MESSAGE.CREATE", "DM_MESSAGE_CREATE":
		return "DM_MESSAGE_CREATE"
	case "DM.MESSAGE.UPDATE", "DM_MESSAGE.UPDATE", "DM_MESSAGE_UPDATE":
		return "DM_MESSAGE_UPDATE"
	case "DM.MESSAGE.DELETE", "DM_MESSAGE.DELETE", "DM_MESSAGE_DELETE":
		return "DM_MESSAGE_DELETE"
	default:
		return eventType
	}
}

func buildTypingBroadcast(event Event) (Event, bool) {
	payload := typingPayload{}
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			return Event{}, false
		}
	}

	if payload.ChannelID == "" {
		payload.ChannelID = event.ChannelID
	}
	if payload.GuildID == "" {
		payload.GuildID = event.GuildID
	}
	if payload.UserID == "" {
		payload.UserID = event.UserID
	}
	if payload.Username == "" {
		payload.Username = "Someone"
	}
	if payload.Timestamp == "" {
		payload.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	if payload.ChannelID == "" || payload.GuildID == "" || payload.UserID == "" {
		return Event{}, false
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return Event{}, false
	}

	return Event{
		Type:      "TYPING_START",
		ChannelID: payload.ChannelID,
		GuildID:   payload.GuildID,
		UserID:    payload.UserID,
		Data:      data,
	}, true
}

func (h *Hub) isGuildMember(ctx context.Context, guildID, userID uuid.UUID) (bool, error) {
	if h.db == nil {
		return false, nil
	}

	var exists bool
	err := h.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM guild_members
			WHERE guild_id = $1 AND user_id = $2
		)`,
		guildID,
		userID,
	).Scan(&exists)
	return exists, err
}

func (h *Hub) isUserActive(ctx context.Context, userID uuid.UUID) (bool, error) {
	if h.db == nil {
		return true, nil
	}

	var exists bool
	err := h.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users WHERE id = $1 AND is_disabled = false
		)`,
		userID,
	).Scan(&exists)
	return exists, err
}

func (h *Hub) isClientActive(ctx context.Context, userID uuid.UUID) (bool, error) {
	if h.userActive == nil {
		return true, nil
	}
	return h.userActive(ctx, userID)
}

func (h *Hub) isOriginAllowed(origin string) bool {
	if origin == "" {
		return true
	}

	if len(h.cfg.AllowedOrigins) == 0 {
		return false
	}

	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		return false
	}

	for _, allowed := range h.cfg.AllowedOrigins {
		if allowed == "*" {
			return true
		}

		parsedAllowed, err := url.Parse(allowed)
		if err != nil {
			continue
		}

		if strings.EqualFold(parsedAllowed.Scheme, parsedOrigin.Scheme) &&
			strings.EqualFold(parsedAllowed.Host, parsedOrigin.Host) {
			return true
		}
	}

	return false
}
