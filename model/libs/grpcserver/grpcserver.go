// Copyright (c) 2024 Second Star Cavalier, LLC.
// Reusable grpcserver components
//
// This package provides a gRPC server with TLS and graceful shutdown.
// Authorization is handled by the Authorizer (see authorizer.go), which is
// injected into the generated service wrapper by the service_manager.
//
// TLS certificates are provided by the servicemanager (auto-obtained via
// X509Mint at startup). Config holds network and request settings.
// REST and static file serving are handled by a separate gateway.

package grpcserver

import (
	"context"
	"crypto"
	"crypto/tls"
	"fmt"
	"net"

	"gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/libs/logger"
	x509wrapper "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/libs/x509"
	protoerrors "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/libs/errors"
	pb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/libs/grpcconfig"
	x509_pb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/libs/x509"
	"go.uber.org/zap"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

// ServiceImplementation is the interface that generated server code must implement.
// It provides the method for registering gRPC services.
type ServiceImplementation interface {
	RegisterServices(grpcServer *grpc.Server) error
}

// GrpcServer manages the gRPC server lifecycle including TLS and graceful
// shutdown. Authorization is managed externally by the service_manager, which
// injects the authorizer into the generated wrapper before creating the server.
type GrpcServer struct {
	config  *pb.Config
	impl    ServiceImplementation
	conn    grpc.ClientConnInterface
	ctx     context.Context
	tlsCert *x509_pb.X509CertChain
	tlsKey  crypto.PrivateKey
}

// New creates a new GrpcServer from a Config.
// tlsCert and tlsKey are required — Listen() will fail if either is nil.
//
// Authorization is managed externally — the caller sets up the authorizer and
// injects it into the wrapper before calling New.
//
// After creating the server, call Listen() to start it.
func New(ctx context.Context, config *pb.Config, conn grpc.ClientConnInterface,
	tlsCert *x509_pb.X509CertChain, tlsKey crypto.PrivateKey,
	impl ServiceImplementation) (*GrpcServer, error) {
	if impl == nil {
		return nil, protoerrors.NewErrMissingInvalidData(nil, "ServiceImplementation is required")
	}
	if config == nil {
		return nil, protoerrors.NewErrMissingInvalidData(nil, "config cannot be nil")
	}

	// Install logger from config.
	if config.Logger != nil {
		if _, err := logger.InstallFromConfig(config.Logger); err != nil {
			zap.L().Error("Failed to install logger", zap.Error(err))
		}
	}

	return &GrpcServer{
		config:  config,
		impl:    impl,
		conn:    conn,
		ctx:     ctx,
		tlsCert: tlsCert,
		tlsKey:  tlsKey,
	}, nil
}

// Listen starts the gRPC server and blocks until the context is cancelled.
// TLS is required — the server refuses to start if no cert was provided.
//
// The caller is responsible for signal handling (e.g., signal.NotifyContext).
func (ps *GrpcServer) Listen(ctx context.Context) error {
	if ps.tlsCert == nil || ps.tlsKey == nil {
		return protoerrors.NewErrMissingInvalidData(nil, "TLS certificate and key are required — plaintext gRPC is not permitted")
	}

	listenAddr := ":8443"
	if ps.config.GetListenAddress() != nil {
		listenAddr = fmt.Sprintf("%s:%d", ps.config.GetListenAddress().Host, ps.config.GetListenAddress().Port)
	}

	var listener net.Listener
	var opts []grpc.ServerOption

	{
		// Build tls.Certificate from the X509CertChain + private key.
		// Chain[0] is the leaf cert, Chain[1:] are intermediates.
		if len(ps.tlsCert.Chain) == 0 {
			return protoerrors.NewErrMissingInvalidData(nil, "X509CertChain must contain at least one certificate")
		}
		leafCert, err := x509wrapper.NewImmutableX509Cert(ps.tlsCert.Chain[0])
		if err != nil {
			return protoerrors.NewErrGeneral(err, "failed to parse leaf certificate")
		}
		tlsCertObj := tls.Certificate{
			Certificate: [][]byte{leafCert.X509Certificate().Raw},
			PrivateKey:  ps.tlsKey,
		}
		// Append intermediate chain certs.
		for _, chainCert := range ps.tlsCert.Chain[1:] {
			immChain, err := x509wrapper.NewImmutableX509Cert(chainCert)
			if err != nil {
				return protoerrors.NewErrGeneral(err, "failed to parse chain certificate")
			}
			tlsCertObj.Certificate = append(tlsCertObj.Certificate, immChain.X509Certificate().Raw)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{tlsCertObj},
			NextProtos:   []string{"h2"},
			MinVersion:   tls.VersionTLS12,
		}

		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}

	{
		lis, err := net.Listen("tcp", listenAddr)
		if err != nil {
			return protoerrors.NewErrGeneral(err, "failed to listen on %s", listenAddr)
		}
		listener = lis
	}
	defer listener.Close()

	if ps.config.MaxRequestSize > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(int(ps.config.MaxRequestSize)))
	}

	// Resource limits from config (defaults set via ConfigDefaults)
	if ps.config.MaxSendMsgSize > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(int(ps.config.MaxSendMsgSize)))
	}
	if ps.config.MaxConcurrentStreams > 0 {
		opts = append(opts, grpc.MaxConcurrentStreams(uint32(ps.config.MaxConcurrentStreams)))
	}

	// Keepalive enforcement
	if ps.config.KeepaliveMinTimeSeconds > 0 {
		opts = append(opts, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             time.Duration(ps.config.KeepaliveMinTimeSeconds) * time.Second,
			PermitWithoutStream: false,
		}))
	}

	grpcServer := grpc.NewServer(opts...)
	if err := ps.impl.RegisterServices(grpcServer); err != nil {
		return fmt.Errorf("failed to register services: %w", err)
	}

	// Register gRPC health check service (unauthenticated, standard grpc.health.v1).
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	zap.L().Info("gRPC server starting",
		zap.String("address", listenAddr),
		zap.Bool("tls", true))

	// Graceful shutdown on context cancellation
	go func() {
		<-ctx.Done()
		shutdownTimeout := time.Duration(ps.config.ShutdownTimeoutSeconds) * time.Second
		if shutdownTimeout <= 0 {
			shutdownTimeout = 30 * time.Second
		}
		zap.L().Info("Shutting down gRPC server", zap.Duration("timeout", shutdownTimeout))
		gracefulStopWithTimeout(grpcServer, shutdownTimeout)
	}()

	return grpcServer.Serve(listener)
}

func gracefulStopWithTimeout(grpcServer *grpc.Server, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		zap.L().Info("gRPC server graceful shutdown complete")
	case <-timer.C:
		zap.L().Warn("gRPC graceful shutdown timed out, forcing stop")
		grpcServer.Stop()
	}
}

// ListenHealthOnly starts a TLS gRPC server that serves only the
// grpc.health.v1 endpoint. Used by services that have no business RPCs
// but still need a health-probeable listener.
func ListenHealthOnly(ctx context.Context, config *pb.Config,
	tlsCert *x509_pb.X509CertChain, tlsKey crypto.PrivateKey) error {
	srv, err := New(ctx, config, nil, tlsCert, tlsKey, &healthOnlyImpl{})
	if err != nil {
		return err
	}
	return srv.Listen(ctx)
}

type healthOnlyImpl struct{}

func (healthOnlyImpl) RegisterServices(_ *grpc.Server) error { return nil }
