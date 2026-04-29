package realtime

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/relay-forge/relay-forge/services/api/internal/config"
)

const EventsChannel = "relayforge.events"

type Event struct {
	Type    string          `json:"type"`
	GuildID *uuid.UUID      `json:"guild_id,omitempty"`
	UserIDs []uuid.UUID     `json:"user_ids,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type Publisher struct {
	client *redis.Client
}

func NewPublisher(cfg config.ValkeyConfig) *Publisher {
	return &Publisher{
		client: redis.NewClient(&redis.Options{
			Addr:     cfg.Addr(),
			Password: cfg.Password,
			DB:       cfg.DB,
			PoolSize: cfg.PoolSize,
		}),
	}
}

func (p *Publisher) Publish(ctx context.Context, event Event) {
	if p == nil || p.client == nil {
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		log.Warn().Err(err).Str("event_type", event.Type).Msg("failed to marshal realtime event")
		return
	}

	if err := p.client.Publish(ctx, EventsChannel, payload).Err(); err != nil {
		log.Warn().Err(err).Str("event_type", event.Type).Msg("failed to publish realtime event")
	}
}

func MustRaw(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		log.Warn().Err(err).Msg("failed to marshal realtime payload")
		return nil
	}
	return data
}
