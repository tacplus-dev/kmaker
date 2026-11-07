package kmaker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	pb "github.com/tacplus-dev/kmaker/tacplus/types/services/kmaker"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// recordingReviewer and recordingPodGetter record the trace_id MintKernelCert
// hands each Kubernetes call.
type recordingReviewer struct {
	fakeReviewer
	traceIDs *[]string
}

func (f recordingReviewer) ReviewToken(ctx context.Context, traceID, token string, audiences []string) (*TokenReviewStatus, error) {
	*f.traceIDs = append(*f.traceIDs, traceID)
	return f.fakeReviewer.ReviewToken(ctx, traceID, token, audiences)
}

type recordingPodGetter struct {
	fakePodGetter
	traceIDs *[]string
}

func (f recordingPodGetter) GetPod(ctx context.Context, traceID, namespace, name string) (*KubernetesPod, error) {
	*f.traceIDs = append(*f.traceIDs, traceID)
	return f.fakePodGetter.GetPod(ctx, traceID, namespace, name)
}

func mintRequest(t *testing.T, traceID string) *pb.MintKernelCertRequest {
	t.Helper()
	return &pb.MintKernelCertRequest{
		K8SToken:    "token",
		CertRequest: testCertRequest(t, false),
		TraceId:     traceID,
	}
}

// TestMintKernelCertPropagatesTraceID asserts the inbound trace_id reaches both
// causal calls, the TokenReview and the Pod lookup, and the handler's log line
// (tacplus USER.md, Distributed Tracing).
func TestMintKernelCertPropagatesTraceID(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	defer zap.ReplaceGlobals(zap.New(core))()

	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	var reviewed, got []string
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	handler, err := NewHandler(cfg, signer,
		recordingReviewer{fakeReviewer: fakeReviewer{status: &TokenReviewStatus{
			Authenticated: true,
			Username:      "system:serviceaccount:schematic:kernel",
			Audiences:     []string{"kmaker"},
			Extra:         tokenReviewPodExtra("kernel-0", "pod-uid-0"),
		}}, traceIDs: &reviewed},
		recordingPodGetter{fakePodGetter: fakePodGetter{pod: testKubernetesPod("schematic", "kernel-0", "pod-uid-0", "kernel", "Running")}, traceIDs: &got})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	if _, err := handler.MintKernelCert(context.Background(), nil, mintRequest(t, traceID)); err != nil {
		t.Fatalf("MintKernelCert: %v", err)
	}
	if len(reviewed) != 1 || reviewed[0] != traceID {
		t.Errorf("TokenReview trace_ids = %q, want [%q]", reviewed, traceID)
	}
	if len(got) != 1 || got[0] != traceID {
		t.Errorf("Pod GET trace_ids = %q, want [%q]", got, traceID)
	}
	signed := logs.FilterMessage("signed kernel certificate").All()
	if len(signed) != 1 {
		t.Fatalf("got %d %q log lines, want 1", len(signed), "signed kernel certificate")
	}
	if v, ok := signed[0].ContextMap()["trace_id"]; !ok || v != traceID {
		t.Errorf("log trace_id = %v (present %v), want %q", v, ok, traceID)
	}
}

// fakeAPIServer answers the TokenReview and Pod GET MintKernelCert makes and
// records each request's headers by path.
type fakeAPIServer struct {
	mu      sync.Mutex
	headers map[string][]http.Header
}

const (
	tokenReviewPath = "/apis/authentication.k8s.io/v1/tokenreviews"
	podPath         = "/api/v1/namespaces/schematic/pods/kernel-0"
)

func (s *fakeAPIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.headers[r.URL.Path] = append(s.headers[r.URL.Path], r.Header.Clone())
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case tokenReviewPath:
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": map[string]any{
			"authenticated": true,
			"audiences":     []string{"kmaker"},
			"user": map[string]any{
				"username": "system:serviceaccount:schematic:kernel",
				"extra":    tokenReviewPodExtra("kernel-0", "pod-uid-0"),
			},
		}})
	case podPath:
		_ = json.NewEncoder(w).Encode(map[string]any{
			"metadata": map[string]any{"namespace": "schematic", "name": "kernel-0", "uid": "pod-uid-0"},
			"spec":     map[string]any{"serviceAccountName": "kernel"},
			"status":   map[string]any{"phase": "Running"},
		})
	default:
		http.NotFound(w, r)
	}
}

// mintAgainstAPIServer runs MintKernelCert with the real HTTP Kubernetes client
// against a fake apiserver and returns the headers of the TokenReview and the
// Pod GET it made.
func mintAgainstAPIServer(t *testing.T, traceID string) (review, pod http.Header) {
	t.Helper()
	api := &fakeAPIServer{headers: map[string][]http.Header{}}
	srv := httptest.NewServer(api)
	defer srv.Close()
	tokenFile := filepath.Join(t.TempDir(), "token")
	writeTokenFile(t, tokenFile, "reviewer-token")
	client := newTestClient(t, srv.URL, tokenFile)

	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	handler, err := NewHandler(cfg, signer, client, client)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	if _, err := handler.MintKernelCert(context.Background(), nil, mintRequest(t, traceID)); err != nil {
		t.Fatalf("MintKernelCert: %v", err)
	}
	if n := len(api.headers[tokenReviewPath]); n != 1 {
		t.Fatalf("apiserver got %d TokenReviews, want 1", n)
	}
	if n := len(api.headers[podPath]); n != 1 {
		t.Fatalf("apiserver got %d Pod GETs, want 1", n)
	}
	return api.headers[tokenReviewPath][0], api.headers[podPath][0]
}

var traceparentRE = regexp.MustCompile(`^00-([0-9a-f]{32})-([0-9a-f]{16})-01$`)

// TestMintKernelCertCarriesTraceparentOnKubernetesCalls: a trace_id that is a
// W3C trace-id (32 lowercase hex, not all zero) travels as traceparent on both
// apiserver calls.
func TestMintKernelCertCarriesTraceparentOnKubernetesCalls(t *testing.T) {
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	review, pod := mintAgainstAPIServer(t, traceID)
	for name, h := range map[string]http.Header{"TokenReview": review, "Pod GET": pod} {
		tp := h.Get("traceparent")
		m := traceparentRE.FindStringSubmatch(tp)
		if m == nil {
			t.Errorf("%s traceparent = %q, want 00-%s-<parent-id>-01", name, tp, traceID)
			continue
		}
		if m[1] != traceID {
			t.Errorf("%s traceparent trace-id = %q, want %q", name, m[1], traceID)
		}
		if m[2] == "0000000000000000" {
			t.Errorf("%s traceparent parent-id is all zero", name)
		}
		if v := h.Get(traceIDHeader); v != "" {
			t.Errorf("%s also carries %s = %q", name, traceIDHeader, v)
		}
	}
}

// TestMintKernelCertCarriesTacplusTraceIDOnKubernetesCalls: any other trace_id
// travels verbatim as X-Tacplus-Trace-Id on both apiserver calls.
func TestMintKernelCertCarriesTacplusTraceIDOnKubernetesCalls(t *testing.T) {
	for _, traceID := range []string{
		"trace-abc-123",
		"4BF92F3577B34DA6A3CE929D0E0E4736", // uppercase is not a W3C trace-id
		"00000000000000000000000000000000", // all zero is invalid in W3C
	} {
		review, pod := mintAgainstAPIServer(t, traceID)
		for name, h := range map[string]http.Header{"TokenReview": review, "Pod GET": pod} {
			if v := h.Get(traceIDHeader); v != traceID {
				t.Errorf("trace_id %q: %s %s = %q, want %q", traceID, name, traceIDHeader, v, traceID)
			}
			if v := h.Get("traceparent"); v != "" {
				t.Errorf("trace_id %q: %s carries traceparent %q", traceID, name, v)
			}
		}
	}
}

// TestMintKernelCertEscapesTraceIDUnfitForAHeader: a trace_id no header can
// carry verbatim (a control character) is percent-encoded, so the trace never
// fails the request.
func TestMintKernelCertEscapesTraceIDUnfitForAHeader(t *testing.T) {
	review, pod := mintAgainstAPIServer(t, "trace\r\nX-Injected: 1")
	for name, h := range map[string]http.Header{"TokenReview": review, "Pod GET": pod} {
		if v, want := h.Get(traceIDHeader), "trace%0D%0AX-Injected%3A+1"; v != want {
			t.Errorf("%s %s = %q, want %q", name, traceIDHeader, v, want)
		}
		if v := h.Get("X-Injected"); v != "" {
			t.Errorf("%s carries an injected header %q", name, v)
		}
	}
}

// TestMintKernelCertWithoutTraceSendsNoTraceHeader: no inbound trace_id, no
// trace header.
func TestMintKernelCertWithoutTraceSendsNoTraceHeader(t *testing.T) {
	review, pod := mintAgainstAPIServer(t, "")
	for name, h := range map[string]http.Header{"TokenReview": review, "Pod GET": pod} {
		if v := h.Get("traceparent"); v != "" {
			t.Errorf("%s carries traceparent %q", name, v)
		}
		if v := h.Get(traceIDHeader); v != "" {
			t.Errorf("%s carries %s %q", name, traceIDHeader, v)
		}
	}
}
