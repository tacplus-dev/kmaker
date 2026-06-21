package kmaker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// newTestClient builds an HTTPKubernetesClient pointed at the given test server
// and reading its bearer token from tokenFile (re-read per request).
func newTestClient(t *testing.T, serverURL, tokenFile string) *HTTPKubernetesClient {
	t.Helper()
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return &HTTPKubernetesClient{
		baseURL:   u,
		tokenFile: tokenFile,
		client:    &http.Client{},
	}
}

func writeTokenFile(t *testing.T, path, token string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
}

// TestReviewTokenReadsTokenPerRequest verifies that ReviewToken re-reads the
// service account token file on each call, so a rotated projected token is
// picked up without restarting the process.
func TestReviewTokenReadsTokenPerRequest(t *testing.T) {
	var captured []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = append(captured, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":{"authenticated":true,"user":{"username":"system:serviceaccount:ns:sa"}}}`))
	}))
	defer srv.Close()

	tokenFile := filepath.Join(t.TempDir(), "token")
	writeTokenFile(t, tokenFile, "first-token")

	c := newTestClient(t, srv.URL, tokenFile)

	if _, err := c.ReviewToken(context.Background(), "sometoken", []string{"aud"}); err != nil {
		t.Fatalf("first ReviewToken: %v", err)
	}

	// Rotate the token on disk.
	writeTokenFile(t, tokenFile, "second-token")

	if _, err := c.ReviewToken(context.Background(), "sometoken", []string{"aud"}); err != nil {
		t.Fatalf("second ReviewToken: %v", err)
	}

	if len(captured) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(captured))
	}
	if captured[0] != "Bearer first-token" {
		t.Fatalf("first request Authorization = %q, want %q", captured[0], "Bearer first-token")
	}
	if captured[1] != "Bearer second-token" {
		t.Fatalf("second request Authorization = %q, want %q (token not re-read per request)", captured[1], "Bearer second-token")
	}
}

// TestGetPodReadsTokenPerRequest verifies the same per-request re-read behavior
// for GetPod.
func TestGetPodReadsTokenPerRequest(t *testing.T) {
	var captured []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = append(captured, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"metadata":{"namespace":"ns","name":"pod"},"spec":{"serviceAccountName":"sa"},"status":{"phase":"Running"}}`))
	}))
	defer srv.Close()

	tokenFile := filepath.Join(t.TempDir(), "token")
	writeTokenFile(t, tokenFile, "first-token")

	c := newTestClient(t, srv.URL, tokenFile)

	if _, err := c.GetPod(context.Background(), "ns", "pod"); err != nil {
		t.Fatalf("first GetPod: %v", err)
	}

	writeTokenFile(t, tokenFile, "second-token")

	if _, err := c.GetPod(context.Background(), "ns", "pod"); err != nil {
		t.Fatalf("second GetPod: %v", err)
	}

	if len(captured) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(captured))
	}
	if captured[0] != "Bearer first-token" {
		t.Fatalf("first request Authorization = %q, want %q", captured[0], "Bearer first-token")
	}
	if captured[1] != "Bearer second-token" {
		t.Fatalf("second request Authorization = %q, want %q (token not re-read per request)", captured[1], "Bearer second-token")
	}
}

// TestReviewTokenFailsClosedOnMissingTokenFile verifies that ReviewToken
// returns an error (and never issues a request) when the token file cannot be
// read, rather than sending an empty bearer token.
func TestReviewTokenFailsClosedOnMissingTokenFile(t *testing.T) {
	var requested bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	c := newTestClient(t, srv.URL, missing)

	if _, err := c.ReviewToken(context.Background(), "sometoken", nil); err == nil {
		t.Fatalf("ReviewToken: expected error for missing token file, got nil")
	}
	if requested {
		t.Fatalf("ReviewToken sent a request despite unreadable token file (fail-open)")
	}
}

// TestGetPodFailsClosedOnMissingTokenFile verifies GetPod fails closed when the
// token file is unreadable.
func TestGetPodFailsClosedOnMissingTokenFile(t *testing.T) {
	var requested bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	c := newTestClient(t, srv.URL, missing)

	if _, err := c.GetPod(context.Background(), "ns", "pod"); err == nil {
		t.Fatalf("GetPod: expected error for missing token file, got nil")
	}
	if requested {
		t.Fatalf("GetPod sent a request despite unreadable token file (fail-open)")
	}
}
