package grpc

import (
	"context"
	"log"

	"github.com/gustavodetoni/pullsing/internal/application"
	redisinfra "github.com/gustavodetoni/pullsing/internal/infrastructure/redis"
)

type RedisRelay struct {
	subscriber *redisinfra.Subscriber
	hub        *Hub
	logger     *log.Logger
}

func NewRedisRelay(subscriber *redisinfra.Subscriber, hub *Hub, logger *log.Logger) *RedisRelay {
	return &RedisRelay{
		subscriber: subscriber,
		hub:        hub,
		logger:     logger,
	}
}

func (r *RedisRelay) Run(ctx context.Context) error {
	return r.subscriber.Run(ctx, func(event application.FlagChangeEvent) error {
		if event.EnvironmentID <= 0 {
			return nil
		}

		r.hub.Publish(EnvironmentEvent{
			EnvironmentID: event.EnvironmentID,
			Revision:      uint64(event.Revision),
		})

		return nil
	})
}
