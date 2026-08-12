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
	loggerchassis "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/chassis/logger"
	kmakergrpc "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/s2sm/grpc/servers/services/kmaker"
	pb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/services/kmaker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func Run(args []string) error {
	// The chassis logger, not a raw zap production logger.
	//
	// Every other service logs through the chassis, which wraps the sink in a
	// redactor so a credential cannot reach the log through a structured FIELD —
	// message-level scrubbing alone does not catch those. kmaker built its logger
	// straight from zap and had no redaction of any kind, which is the
	// worst service in the fleet to leave uncovered: it holds the signing CA and
	// mints keys, so its error paths are the ones most likely to carry key
	// material. It is hand-written rather than generated, which is how it slipped
	// past the sweep that fixed the generated services.
	//
	// Installed before the config is read so early failures are redacted too, then
	// re-installed from the service's own logger config once that is available.
	logger, err := loggerchassis.InstallFromConfig(nil)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	zap.ReplaceGlobals(logger)
	defer logger.Sync() //nolint:errcheck

	cfg, err := LoadConfigFile(ResolveConfigPath(args))
	if err != nil {
		return err
	}
	// Apply annotated proto defaults before ANY field is read. kmaker does not
	// use the generated ServiceManager (which does this in its own bootstrap),
	// so nothing else applies them. Without this GetHealthProbePort() returns 0,
	// the `> 0` guard below skips, and the plaintext probe never binds — the
	// exact failure kernel hit for the same reason.
	pb.ConfigDefaults(cfg)

	// Re-install now that the service's own logger config (level, format, output
	// paths) is known and its defaults are applied. Redaction is unconditional in
	// the chassis, so this changes verbosity and shape, never whether credentials
	// are scrubbed.
	if configured, err := loggerchassis.InstallFromConfig(cfg.GetLogger()); err != nil {
		zap.L().Warn("keeping the bootstrap logger; configured logger failed to install",
			zap.Error(err))
	} else {
		zap.ReplaceGlobals(configured)
		logger = configured
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
