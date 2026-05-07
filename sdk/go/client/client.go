package client

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gustavodetoni/pullsing/sdk/go/cache"
	"github.com/gustavodetoni/pullsing/sdk/go/stream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	EnvAPIKey   string
	Addr        string
	DialOptions []grpc.DialOption
	Backoff     stream.BackoffConfig
	Logger      stream.Logger
}

type Health struct {
	Connected    bool
	LastRevision uint64
	LastError    error
	LastSyncTime time.Time
}

type Client struct {
	store  *cache.Store
	runner *stream.Runner
	conn   *grpc.ClientConn

	cancel context.CancelFunc
	done   chan struct{}

	stateMu sync.RWMutex
	health  Health
}

func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.EnvAPIKey == "" {
		return nil, errors.New("pullsing sdk: env api key is required")
	}
	if cfg.Addr == "" {
		return nil, errors.New("pullsing sdk: addr is required")
	}

	dialOptions := append([]grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}, cfg.DialOptions...)

	conn, err := grpc.DialContext(ctx, cfg.Addr, dialOptions...)
	if err != nil {
		return nil, err
	}

	return NewClientWithService(stream.NewGRPCService(conn), cfg, conn), nil
}

func NewClientWithService(service stream.Service, cfg Config, conn *grpc.ClientConn) *Client {
	client := &Client{
		store: cache.NewStore(),
		conn:  conn,
		done:  make(chan struct{}),
	}

	runner := stream.NewRunner(service, client.store, client, stream.RunnerConfig{
		EnvAPIKey: cfg.EnvAPIKey,
		Backoff:   cfg.Backoff,
		Logger:    cfg.Logger,
	})
	client.runner = runner

	runCtx, cancel := context.WithCancel(context.Background())
	client.cancel = cancel
	go func() {
		defer close(client.done)
		_ = client.runner.Run(runCtx)
	}()

	return client
}

func (c *Client) Enabled(key string) bool {
	return c.store.Enabled(key)
}

func (c *Client) Revision() uint64 {
	return c.store.Revision()
}

func (c *Client) Health() Health {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.health
}

func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}

	<-c.done

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

func (c *Client) OnStateChange(state stream.State) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.health = Health{
		Connected:    state.Connected,
		LastRevision: state.Revision,
		LastError:    state.LastError,
		LastSyncTime: state.LastSyncTime,
	}
}
