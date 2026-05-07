package grpcapi

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/gustavodetoni/pullsing/internal/application"
	"github.com/gustavodetoni/pullsing/internal/domain"
	grpcinfra "github.com/gustavodetoni/pullsing/internal/infrastructure/grpc"
	pullsingv1 "github.com/gustavodetoni/pullsing/proto/gen/go/pullsing/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGetSnapshotRejectsInvalidEnvAPIKey(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, &fakeSDKService{
		getSnapshotErr: application.ErrUnauthorized,
	}, grpcinfra.NewHub(1))
	defer cleanup()

	_, err := client.GetSnapshot(context.Background(), &pullsingv1.GetSnapshotRequest{EnvApiKey: "invalid"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetSnapshot() code = %s, want %s", status.Code(err), codes.Unauthenticated)
	}
}

func TestStreamUpdatesSendsBacklogAndRealtimeChanges(t *testing.T) {
	t.Parallel()

	service := &fakeSDKService{
		environment: domain.Environment{ID: 42},
	}
	service.setUpdates(1, []application.Update{
		{
			Revision: 2,
			Mutations: []application.Mutation{
				{
					Type: application.MutationTypeUpsert,
					Key:  "checkout-redesign",
					Flag: application.SnapshotFlag{
						Key:       "checkout-redesign",
						Enabled:   true,
						BoolValue: true,
					},
				},
			},
		},
	})
	service.setUpdates(2, []application.Update{
		{
			Revision: 3,
			Mutations: []application.Mutation{
				{
					Type: application.MutationTypeDelete,
					Key:  "legacy-banner",
				},
			},
		},
	})

	hub := grpcinfra.NewHub(2)
	client, cleanup := newTestClient(t, service, hub)
	defer cleanup()

	stream, err := client.StreamUpdates(context.Background(), &pullsingv1.StreamUpdatesRequest{
		EnvApiKey:     "psk_valid",
		SinceRevision: 1,
	})
	if err != nil {
		t.Fatalf("StreamUpdates() error = %v", err)
	}

	backlog, err := stream.Recv()
	if err != nil {
		t.Fatalf("stream.Recv() backlog error = %v", err)
	}
	if backlog.GetRevision() != 2 {
		t.Fatalf("backlog revision = %d, want 2", backlog.GetRevision())
	}

	hub.Publish(grpcinfra.EnvironmentEvent{EnvironmentID: 42, Revision: 3})

	realtime, err := stream.Recv()
	if err != nil {
		t.Fatalf("stream.Recv() realtime error = %v", err)
	}
	if realtime.GetRevision() != 3 {
		t.Fatalf("realtime revision = %d, want 3", realtime.GetRevision())
	}
	if len(realtime.GetMutations()) != 1 || realtime.GetMutations()[0].GetType() != pullsingv1.MutationType_MUTATION_TYPE_DELETE {
		t.Fatalf("realtime mutations = %#v, want delete", realtime.GetMutations())
	}
}

func newTestClient(t *testing.T, service *fakeSDKService, hub *grpcinfra.Hub) (pullsingv1.SDKServiceClient, func()) {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pullsingv1.RegisterSDKServiceServer(server, NewSDKServer(service, hub))

	go func() {
		_ = server.Serve(listener)
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithInsecure(),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		server.Stop()
		_ = listener.Close()
	}

	return pullsingv1.NewSDKServiceClient(conn), cleanup
}

type fakeSDKService struct {
	mu             sync.Mutex
	environment    domain.Environment
	getSnapshot    application.Snapshot
	getSnapshotErr error
	updateBatches  map[uint64][]application.Update
}

func (f *fakeSDKService) AuthenticateEnvironment(_ context.Context, envAPIKey string) (domain.Environment, error) {
	if envAPIKey == "" || envAPIKey == "invalid" {
		return domain.Environment{}, application.ErrUnauthorized
	}
	return f.environment, nil
}

func (f *fakeSDKService) GetSnapshot(_ context.Context, envAPIKey string) (application.Snapshot, error) {
	if f.getSnapshotErr != nil {
		return application.Snapshot{}, f.getSnapshotErr
	}
	if envAPIKey == "" || envAPIKey == "invalid" {
		return application.Snapshot{}, application.ErrUnauthorized
	}
	return f.getSnapshot, nil
}

func (f *fakeSDKService) ListUpdatesSince(_ context.Context, _ int64, sinceRevision uint64) ([]application.Update, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.updateBatches == nil {
		return nil, nil
	}

	updates := f.updateBatches[sinceRevision]
	if updates == nil {
		return nil, nil
	}

	out := make([]application.Update, len(updates))
	copy(out, updates)
	return out, nil
}

func (f *fakeSDKService) setUpdates(sinceRevision uint64, updates []application.Update) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.updateBatches == nil {
		f.updateBatches = make(map[uint64][]application.Update)
	}
	f.updateBatches[sinceRevision] = updates
}

var _ sdkService = (*fakeSDKService)(nil)
