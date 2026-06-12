package kmaker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os/signal"
	"syscall"
	"time"

	kmakergrpc "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/s2sm/grpc/servers/services/kmaker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	signer, err := NewSigner(cfg)
	if err != nil {
		return fmt.Errorf("signer: %w", err)
	}
	reviewer, err := NewInClusterTokenReviewer()
	if err != nil {
		return fmt.Errorf("kubernetes token reviewer: %w", err)
	}
	handler, err := NewHandler(cfg, signer, reviewer)
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
	reflection.Register(server)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
