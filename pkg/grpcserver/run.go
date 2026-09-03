package grpcserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"rgb-game/pkg/logger"
	"syscall"

	"google.golang.org/grpc"
)

// Run binds to port, starts s in a background goroutine, and blocks until ctx
// is cancelled or SIGINT/SIGTERM is received. It then calls GracefulStop and
// returns any error from the initial Listen call or from Serve.
func Run(ctx context.Context, s *grpc.Server, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("grpcserver: listen on :%d: %w", port, err)
	}
	logger.Infof("gRPC server listening on %v", lis.Addr())

	serveErr := make(chan error, 1)
	go func() {
		if err := s.Serve(lis); err != nil {
			serveErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Infof("Received %s, shutting down gracefully…", sig)
	case <-ctx.Done():
		logger.Info("Context cancelled, shutting down…")
	case err := <-serveErr:
		return fmt.Errorf("grpcserver: serve: %w", err)
	}

	s.GracefulStop()
	return nil
}
