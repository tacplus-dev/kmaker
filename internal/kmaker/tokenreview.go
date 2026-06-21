package kmaker

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultSATokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultSACAFile    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

var ErrNoKubernetesConfig = errors.New("kubernetes API endpoint is not configured")

type TokenReviewer interface {
	ReviewToken(ctx context.Context, token string, audiences []string) (*TokenReviewStatus, error)
}

type PodGetter interface {
	GetPod(ctx context.Context, namespace, name string) (*KubernetesPod, error)
}

type TokenReviewStatus struct {
	Authenticated bool
	Username      string
	UID           string
	Groups        []string
	Audiences     []string
	Error         string
	Extra         map[string][]string
}

type KubernetesPod struct {
	Namespace          string
	Name               string
	UID                string
	ServiceAccountName string
	Phase              string
}

type HTTPKubernetesClient struct {
	baseURL *url.URL
	token   string
	client  *http.Client
}

type HTTPTokenReviewer = HTTPKubernetesClient

func NewInClusterKubernetesClient() (*HTTPKubernetesClient, error) {
	endpoint := os.Getenv("KMAKER_KUBE_API_URL")
	if endpoint == "" {
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		port := os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS")
		if port == "" {
			port = os.Getenv("KUBERNETES_SERVICE_PORT")
		}
		if host == "" || port == "" {
			return nil, ErrNoKubernetesConfig
		}
		endpoint = "https://" + host + ":" + port
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse kubernetes API URL: %w", err)
	}

	tokenFile := getenvDefault("KMAKER_KUBE_TOKEN_FILE", defaultSATokenFile)
	tokenBytes, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("read kubernetes service account token %s: %w", tokenFile, err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	caFile := getenvDefault("KMAKER_KUBE_CA_FILE", defaultSACAFile)
	if caBytes, err := os.ReadFile(caFile); err == nil && len(caBytes) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caBytes) {
			return nil, fmt.Errorf("parse kubernetes service account CA %s", caFile)
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}

	return &HTTPKubernetesClient{
		baseURL: u,
		token:   strings.TrimSpace(string(tokenBytes)),
		client:  &http.Client{Transport: transport, Timeout: 10 * time.Second},
	}, nil
}

func NewInClusterTokenReviewer() (*HTTPTokenReviewer, error) {
	return NewInClusterKubernetesClient()
}

func getenvDefault(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func (r *HTTPKubernetesClient) ReviewToken(ctx context.Context, token string, audiences []string) (*TokenReviewStatus, error) {
	if r == nil || r.client == nil || r.baseURL == nil {
		return nil, ErrNoKubernetesConfig
	}
	body := tokenReviewRequest{
		APIVersion: "authentication.k8s.io/v1",
		Kind:       "TokenReview",
		Spec: tokenReviewSpec{
			Token:     token,
			Audiences: audiences,
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal tokenreview: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.urlFor("/apis/authentication.k8s.io/v1/tokenreviews"), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create tokenreview request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tokenreview request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("tokenreview status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out tokenReviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode tokenreview response: %w", err)
	}
	return &TokenReviewStatus{
		Authenticated: out.Status.Authenticated,
		Username:      out.Status.User.Username,
		UID:           out.Status.User.UID,
		Groups:        out.Status.User.Groups,
		Audiences:     out.Status.Audiences,
		Error:         out.Status.Error,
		Extra:         out.Status.User.Extra,
	}, nil
}

func (r *HTTPKubernetesClient) GetPod(ctx context.Context, namespace, name string) (*KubernetesPod, error) {
	if r == nil || r.client == nil || r.baseURL == nil {
		return nil, ErrNoKubernetesConfig
	}
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("pod namespace and name are required")
	}
	p := "/api/v1/namespaces/" + url.PathEscape(namespace) + "/pods/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.urlFor(p), nil)
	if err != nil {
		return nil, fmt.Errorf("create pod get request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pod get request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("pod get status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out podResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode pod response: %w", err)
	}
	return &KubernetesPod{
		Namespace:          out.Metadata.Namespace,
		Name:               out.Metadata.Name,
		UID:                out.Metadata.UID,
		ServiceAccountName: out.Spec.ServiceAccountName,
		Phase:              out.Status.Phase,
	}, nil
}

func (r *HTTPKubernetesClient) urlFor(p string) string {
	u := *r.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + p
	return u.String()
}

func ParseServiceAccountUsername(username string) (ServiceAccount, bool) {
	parts := strings.Split(username, ":")
	if len(parts) != 4 {
		return ServiceAccount{}, false
	}
	if parts[0] != "system" || parts[1] != "serviceaccount" || parts[2] == "" || parts[3] == "" {
		return ServiceAccount{}, false
	}
	return ServiceAccount{Namespace: parts[2], Name: parts[3]}, true
}

type tokenReviewRequest struct {
	APIVersion string          `json:"apiVersion"`
	Kind       string          `json:"kind"`
	Spec       tokenReviewSpec `json:"spec"`
}

type tokenReviewSpec struct {
	Token     string   `json:"token"`
	Audiences []string `json:"audiences,omitempty"`
}

type tokenReviewResponse struct {
	Status tokenReviewResponseStatus `json:"status"`
}

type tokenReviewResponseStatus struct {
	Authenticated bool            `json:"authenticated"`
	User          tokenReviewUser `json:"user"`
	Audiences     []string        `json:"audiences"`
	Error         string          `json:"error"`
}

type tokenReviewUser struct {
	Username string              `json:"username"`
	UID      string              `json:"uid"`
	Groups   []string            `json:"groups"`
	Extra    map[string][]string `json:"extra"`
}

type podResponse struct {
	Metadata podMetadata `json:"metadata"`
	Spec     podSpec     `json:"spec"`
	Status   podStatus   `json:"status"`
}

type podMetadata struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

type podSpec struct {
	ServiceAccountName string `json:"serviceAccountName"`
}

type podStatus struct {
	Phase string `json:"phase"`
}
