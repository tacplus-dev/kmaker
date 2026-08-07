package kmaker

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/chassis/grpcserver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestHealthProbeListenerBindsAndServes(t *testing.T) {
	// Pick a free port to avoid collisions with any running service.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := lis.Addr().(*net.TCPAddr).Port
	lis.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hs := grpcserver.NewHealthServer()

	errCh := make(chan error, 1)
	go func() {
		errCh <- grpcserver.ListenHealthProbe(ctx, int32(port), hs)
	}()

	// Wait for the probe server to become connectable.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	var conn *grpc.ClientConn
	for i := 0; i < 20; i++ {
		conn, err = grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dial health probe: %v", err)
	}
	defer conn.Close()

	client := healthpb.NewHealthClient(conn)

	// The health server starts NOT_SERVING.
	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("expected NOT_SERVING before SetServingStatus, got %v", resp.Status)
	}

	// Flip to SERVING (simulates what Run does after wiring the main server).
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	resp, err = client.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check after SERVING: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("expected SERVING, got %v", resp.Status)
	}

	// Shutdown: cancel context, probe server should exit cleanly.
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("probe server returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("probe server did not shut down within 5s")
	}
}

func TestHealthProbeDisabledWhenPortZero(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hs := grpcserver.NewHealthServer()

	// port=0 should return nil immediately without binding.
	err := grpcserver.ListenHealthProbe(ctx, 0, hs)
	if err != nil {
		t.Fatalf("expected nil for port=0, got: %v", err)
	}
}
