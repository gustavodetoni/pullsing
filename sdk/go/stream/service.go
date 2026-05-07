package stream

import (
	"context"
	"io"

	pullsingv1 "github.com/gustavodetoni/pullsing/proto/gen/go/pullsing/v1"
	"google.golang.org/grpc"
)

type Service interface {
	GetSnapshot(ctx context.Context, envAPIKey string) (*pullsingv1.Snapshot, error)
	StreamUpdates(ctx context.Context, envAPIKey string, sinceRevision uint64) (UpdateStream, error)
}

type UpdateStream interface {
	Recv() (*pullsingv1.Update, error)
	Close() error
}

type GRPCService struct {
	client pullsingv1.SDKServiceClient
}

func NewGRPCService(conn grpc.ClientConnInterface) *GRPCService {
	return &GRPCService{
		client: pullsingv1.NewSDKServiceClient(conn),
	}
}

func (s *GRPCService) GetSnapshot(ctx context.Context, envAPIKey string) (*pullsingv1.Snapshot, error) {
	return s.client.GetSnapshot(ctx, &pullsingv1.GetSnapshotRequest{EnvApiKey: envAPIKey})
}

func (s *GRPCService) StreamUpdates(ctx context.Context, envAPIKey string, sinceRevision uint64) (UpdateStream, error) {
	stream, err := s.client.StreamUpdates(ctx, &pullsingv1.StreamUpdatesRequest{
		EnvApiKey:     envAPIKey,
		SinceRevision: sinceRevision,
	})
	if err != nil {
		return nil, err
	}

	return &grpcUpdateStream{stream: stream}, nil
}

type grpcUpdateStream struct {
	stream grpc.ServerStreamingClient[pullsingv1.Update]
}

func (s *grpcUpdateStream) Recv() (*pullsingv1.Update, error) {
	return s.stream.Recv()
}

func (s *grpcUpdateStream) Close() error {
	return s.stream.CloseSend()
}

func isExpectedStreamEnd(err error) bool {
	return err == nil || err == io.EOF || err == context.Canceled
}
