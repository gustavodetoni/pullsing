package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gustavodetoni/pullsing/internal/application"
	redisv9 "github.com/redis/go-redis/v9"
)

type Subscriber struct {
	client  *redisv9.Client
	channel string
}

func NewSubscriber(client *redisv9.Client, channel string) *Subscriber {
	if channel == "" {
		channel = DefaultChannel
	}

	return &Subscriber{
		client:  client,
		channel: channel,
	}
}

func (s *Subscriber) Run(ctx context.Context, handler func(application.FlagChangeEvent) error) error {
	pubsub := s.client.Subscribe(ctx, s.channel)
	defer pubsub.Close()

	if _, err := pubsub.Receive(ctx); err != nil {
		return fmt.Errorf("subscribe redis channel: %w", err)
	}

	channel := pubsub.Channel(redisv9.WithChannelSize(128))
	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-channel:
			if !ok {
				if errors.Is(ctx.Err(), context.Canceled) {
					return nil
				}
				return errors.New("redis pubsub channel closed")
			}

			var event application.FlagChangeEvent
			if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
				continue
			}

			if err := handler(event); err != nil {
				return err
			}
		}
	}
}
