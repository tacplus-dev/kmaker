package kmaker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os/signal"
	"syscall"
	"time"

	"gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/chassis/grpcserver"
	kmakergrpc "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/s2sm/grpc/servers/services/kmaker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func Run(args []string) error {
	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	zap.ReplaceGlobals(logger)
	defer logger.Sync() //nolint:errcheck

	cfg, err := LoadConfigFile(ResolveConfigPath(args))
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create the health server EARLY so the plaintext probe can bind before
	// any kernel-dependent init. Starts NOT_SERVING; flipped to SERVING once
	// the main gRPC server is ready.
	healthServer := grpcserver.NewHealthServer()
	if port := cfg.GetHealthProbePort(); port > 0 {
		go func() {
			if err := grpcserver.ListenHealthProbe(ctx, port, healthServer); err != nil && ctx.Err() == nil {
				zap.L().Error("health probe server error", zap.Error(err))
			}
		}()
	}

	signer, err := NewSigner(cfg)
	if err != nil {
		return fmt.Errorf("signer: %w", err)
	}
	kube, err := NewInClusterKubernetesClient()
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}
	handler, err := NewHandler(cfg, signer, kube, kube)
	if err != nil {
		return fmt.Errorf("handler: %w", err)
	}

	addr := ListenAddress(cfg)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	defer lis.Close()

	opts := []grpc.ServerOption{}
	if cfg.GetTls() != nil && len(cfg.GetTls().GetChain()) > 0 && cfg.GetTls().GetKey() != nil {
		cert, err := TLSCertificateFromChain(cfg.GetTls())
		if err != nil {
			return fmt.Errorf("kmaker serving tls: %w", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		})))
	} else {
		zap.L().Warn("kmaker starting without serving TLS; this is intended only for local/dev bootstrap")
	}

	server := grpc.NewServer(opts...)
	wrapper := kmakergrpc.NewKmaker(handler)
	if err := wrapper.RegisterServices(server); err != nil {
		return fmt.Errorf("register kmaker service: %w", err)
	}
	// Share the same health server on the main gRPC listener so TLS clients
	// can also query grpc.health.v1.
	healthpb.RegisterHealthServer(server, healthServer)
	reflection.Register(server)

	// Flip to SERVING now that the main server is fully wired.
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	errCh := make(chan error, 1)
	go func() {
		zap.L().Info("starting kmaker", zap.String("addr", addr), zap.String("cluster_name", cfg.GetClusterName()))
		if err := server.Serve(lis); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(15 * time.Second):
		server.Stop()
		<-stopped
	}
	return nil
}
