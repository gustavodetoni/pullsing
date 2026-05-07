package grpcapi

import (
	"context"
	"errors"
	"log"
	"net"
	"time"

	"github.com/gustavodetoni/pullsing/internal/application"
	"github.com/gustavodetoni/pullsing/internal/domain"
	"github.com/gustavodetoni/pullsing/internal/infrastructure/config"
	grpcinfra "github.com/gustavodetoni/pullsing/internal/infrastructure/grpc"
	pullsingv1 "github.com/gustavodetoni/pullsing/proto/gen/go/pullsing/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type sdkService interface {
	AuthenticateEnvironment(ctx context.Context, envAPIKey string) (domain.Environment, error)
	GetSnapshot(ctx context.Context, envAPIKey string) (application.Snapshot, error)
	ListUpdatesSince(ctx context.Context, environmentID int64, sinceRevision uint64) ([]application.Update, error)
}

type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	config     config.Config
	logger     *log.Logger
	relay      *grpcinfra.RedisRelay
}

type SDKServer struct {
	pullsingv1.UnimplementedSDKServiceServer
	service sdkService
	hub     *grpcinfra.Hub
}

func New(cfg config.Config, logger *log.Logger, service sdkService, hub *grpcinfra.Hub, relay *grpcinfra.RedisRelay) (*Server, error) {
	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer()
	pullsingv1.RegisterSDKServiceServer(grpcServer, NewSDKServer(service, hub))

	return &Server{
		grpcServer: grpcServer,
		listener:   listener,
		config:     cfg,
		logger:     logger,
		relay:      relay,
	}, nil
}

func NewSDKServer(service sdkService, hub *grpcinfra.Hub) *SDKServer {
	return &SDKServer{
		service: service,
		hub:     hub,
	}
}

func (s *Server) Run(ctx context.Context) error {
	expected := 1
	if s.relay != nil {
		expected++
	}

	errCh := make(chan error, expected)

	if s.relay != nil {
		go func() {
			errCh <- s.relay.Run(ctx)
		}()
	}

	go func() {
		if s.logger != nil {
			s.logger.Printf("grpc server listening on %s", s.listener.Addr())
		}
		errCh <- s.grpcServer.Serve(s.listener)
	}()

	select {
	case <-ctx.Done():
		done := make(chan struct{})
		go func() {
			defer close(done)
			s.grpcServer.GracefulStop()
		}()

		select {
		case <-done:
		case <-time.After(s.config.ShutdownTimeout):
			s.grpcServer.Stop()
		}

		for i := 0; i < expected; i++ {
			<-errCh
		}

		return nil
	case err := <-errCh:
		s.grpcServer.Stop()
		if err == nil || errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	}
}

func (s *SDKServer) GetSnapshot(ctx context.Context, request *pullsingv1.GetSnapshotRequest) (*pullsingv1.Snapshot, error) {
	snapshot, err := s.service.GetSnapshot(ctx, request.GetEnvApiKey())
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return &pullsingv1.Snapshot{
		Revision: snapshot.Revision,
		Flags:    mapProtoFlags(snapshot.Flags),
	}, nil
}

func (s *SDKServer) StreamUpdates(request *pullsingv1.StreamUpdatesRequest, stream grpc.ServerStreamingServer[pullsingv1.Update]) error {
	environment, err := s.service.AuthenticateEnvironment(stream.Context(), request.GetEnvApiKey())
	if err != nil {
		return mapGRPCError(err)
	}

	subscription := s.hub.Subscribe(environment.ID)
	defer subscription.Close()

	lastRevision := request.GetSinceRevision()
	if err := s.sendPendingUpdates(stream.Context(), stream, environment.ID, &lastRevision); err != nil {
		return err
	}

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case event, ok := <-subscription.Events():
			if !ok {
				if subscription.Slow() {
					return status.Error(codes.ResourceExhausted, "client stream is too slow")
				}
				return nil
			}
			if event.Revision <= lastRevision {
				continue
			}
			if err := s.sendPendingUpdates(stream.Context(), stream, environment.ID, &lastRevision); err != nil {
				return err
			}
		}
	}
}

func (s *SDKServer) sendPendingUpdates(ctx context.Context, stream grpc.ServerStreamingServer[pullsingv1.Update], environmentID int64, lastRevision *uint64) error {
	updates, err := s.service.ListUpdatesSince(ctx, environmentID, *lastRevision)
	if err != nil {
		return mapGRPCError(err)
	}

	for _, update := range updates {
		if err := stream.Send(mapProtoUpdate(update)); err != nil {
			return err
		}
		*lastRevision = update.Revision
	}

	return nil
}

func mapGRPCError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, application.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, application.ErrInvalidCursor):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, application.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func mapProtoFlags(flags []application.SnapshotFlag) []*pullsingv1.Flag {
	if len(flags) == 0 {
		return nil
	}

	out := make([]*pullsingv1.Flag, 0, len(flags))
	for _, flag := range flags {
		out = append(out, &pullsingv1.Flag{
			Key:       flag.Key,
			Type:      pullsingv1.FlagType_FLAG_TYPE_BOOL,
			Enabled:   flag.Enabled,
			BoolValue: flag.BoolValue,
		})
	}

	return out
}

func mapProtoUpdate(update application.Update) *pullsingv1.Update {
	protoUpdate := &pullsingv1.Update{
		Revision: update.Revision,
	}

	if len(update.Mutations) == 0 {
		return protoUpdate
	}

	protoUpdate.Mutations = make([]*pullsingv1.FlagMutation, 0, len(update.Mutations))
	for _, mutation := range update.Mutations {
		protoMutation := &pullsingv1.FlagMutation{
			Key: mutation.Key,
		}

		switch mutation.Type {
		case application.MutationTypeUpsert:
			protoMutation.Type = pullsingv1.MutationType_MUTATION_TYPE_UPSERT
			protoMutation.Flag = &pullsingv1.Flag{
				Key:       mutation.Flag.Key,
				Type:      pullsingv1.FlagType_FLAG_TYPE_BOOL,
				Enabled:   mutation.Flag.Enabled,
				BoolValue: mutation.Flag.BoolValue,
			}
		case application.MutationTypeDelete:
			protoMutation.Type = pullsingv1.MutationType_MUTATION_TYPE_DELETE
		default:
			protoMutation.Type = pullsingv1.MutationType_MUTATION_TYPE_UNSPECIFIED
		}

		protoUpdate.Mutations = append(protoUpdate.Mutations, protoMutation)
	}

	return protoUpdate
}
