package hub

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/relay-forge/relay-forge/services/realtime/internal/config"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Event types sent over WebSocket
const (
	EventMessage       = "message.create"
	EventMessageUpdate = "message.update"
	EventMessageDelete = "message.delete"
	EventTypingStart   = "typing.start"
	EventTypingStop    = "typing.stop"
	EventPresence      = "presence.update"
	EventReadState     = "read_state.update"
	EventGuildUpdate   = "guild.update"
	EventChannelUpdate = "channel.update"
	EventMemberJoin    = "member.join"
	EventMemberLeave   = "member.leave"
)

type Event struct {
	Type      string          `json:"type"`
	ChannelID string          `json:"channel_id,omitempty"`
	GuildID   string          `json:"guild_id,omitempty"`
	UserID    string          `json:"user_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
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
	clients    map[uuid.UUID]map[*Client]bool // userID -> set of clients
	guilds     map[uuid.UUID]map[*Client]bool // guildID -> set of clients
	register   chan *Client
	unregister chan *Client
	broadcast  chan *guildMessage
	mu         sync.RWMutex
	quit       chan struct{}
}

type guildMessage struct {
	guildID uuid.UUID
	data    []byte
}

func New(cfg *config.Config) *Hub {
	return &Hub{
		cfg:        cfg,
		clients:    make(map[uuid.UUID]map[*Client]bool),
		guilds:     make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
		broadcast:  make(chan *guildMessage, 4096),
		quit:       make(chan struct{}),
	}
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
}

func (h *Hub) BroadcastToGuild(guildID uuid.UUID, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal broadcast event")
		return
	}
	h.broadcast <- &guildMessage{guildID: guildID, data: data}
}

func (h *Hub) SendToUser(userID uuid.UUID, event Event) {
	data, err := json.Marshal(event)
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

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	userID, err := h.validateToken(tokenStr)
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
		if gid, err := uuid.Parse(gidStr); err == nil {
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

func (h *Hub) validateToken(tokenStr string) (uuid.UUID, error) {
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

	return uuid.Parse(sub)
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
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
	defer func() {
		ticker.Stop()
		c.conn.Close()
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
		}
	}
}

func (c *Client) handleEvent(event Event) {
	switch event.Type {
	case EventTypingStart, EventTypingStop:
		if gid, err := uuid.Parse(event.GuildID); err == nil {
			c.hub.BroadcastToGuild(gid, event)
		}
	case EventReadState:
		// Read state updates are per-user, echoed back
		c.hub.SendToUser(c.userID, event)
	case EventPresence:
		for gid := range c.guildIDs {
			c.hub.BroadcastToGuild(gid, event)
		}
	}
}
