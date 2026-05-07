package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gustavodetoni/pullsing/internal/application"
	redisv9 "github.com/redis/go-redis/v9"
)

const DefaultChannel = "pullsing.environment-updates"

type Publisher struct {
	client  *redisv9.Client
	channel string
}

func NewClient(addr string) *redisv9.Client {
	return redisv9.NewClient(&redisv9.Options{
		Addr: addr,
	})
}

func NewPublisher(client *redisv9.Client, channel string) *Publisher {
	if channel == "" {
		channel = DefaultChannel
	}

	return &Publisher{
		client:  client,
		channel: channel,
	}
}

func (p *Publisher) PublishFlagChange(ctx context.Context, event application.FlagChangeEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal redis flag event: %w", err)
	}

	if err := p.client.Publish(ctx, p.channel, payload).Err(); err != nil {
		return fmt.Errorf("publish redis flag event: %w", err)
	}

	return nil
}
