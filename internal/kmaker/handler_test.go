package kmaker

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	stdx509 "crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/url"
	"testing"
	"time"

	keypb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/sys/key"
	x509pb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/sys/x509"
	pb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/services/kmaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeReviewer struct {
	status *TokenReviewStatus
	err    error
}

func (f fakeReviewer) ReviewToken(context.Context, string, []string) (*TokenReviewStatus, error) {
	return f.status, f.err
}

type fakePodGetter struct {
	pod *KubernetesPod
	err error
}

func (f fakePodGetter) GetPod(context.Context, string, string) (*KubernetesPod, error) {
	return f.pod, f.err
}

func TestParseServiceAccountUsername(t *testing.T) {
	sa, ok := ParseServiceAccountUsername("system:serviceaccount:cavalier:kernel")
	if !ok {
		t.Fatal("expected service account username to parse")
	}
	if sa.Namespace != "cavalier" || sa.Name != "kernel" {
		t.Fatalf("unexpected service account: %#v", sa)
	}
	if _, ok := ParseServiceAccountUsername("alice@example.com"); ok {
		t.Fatal("non-service-account username parsed")
	}
}

func TestHandlerSignsKernelCert(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	handler, err := NewHandler(cfg, signer, fakeReviewer{status: &TokenReviewStatus{
		Authenticated: true,
		Username:      "system:serviceaccount:cavalier:kernel",
		UID:           "uid-1",
		Audiences:     []string{"kmaker"},
		Extra:         tokenReviewPodExtra("kernel-0", "pod-uid-0"),
	}}, fakePodGetter{pod: testKubernetesPod("cavalier", "kernel-0", "pod-uid-0", "kernel", "Running")})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := &pb.MintKernelCertRequest{
		K8SToken:    "token",
		CertRequest: testCertRequest(t, false),
	}
	chain, err := handler.MintKernelCert(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("MintKernelCert: %v", err)
	}
	if got := len(chain.GetChain()); got != 2 {
		t.Fatalf("expected leaf + CA chain, got %d certs", got)
	}

	leaf, err := parseCertProto(chain.GetChain()[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	assertURI(t, leaf.URIs, "spiffe://local/dev/kernel")
	assertURI(t, leaf.URIs, "spiffe://local/dev/kernel/kernel-0")
	if leaf.IsCA {
		t.Fatal("expected leaf certificate")
	}
	if _, err := leaf.Verify(stdx509.VerifyOptions{
		Roots:       certPool(t, cfg.GetSigningCa().GetChain()[0]),
		CurrentTime: time.Now(),
		KeyUsages:   []stdx509.ExtKeyUsage{stdx509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Fatalf("verify leaf: %v", err)
	}
}

func TestHandlerRejectsCallerSuppliedKernelIdentity(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	handler, err := NewHandler(cfg, signer, fakeReviewer{status: &TokenReviewStatus{
		Authenticated: true,
		Username:      "system:serviceaccount:cavalier:kernel",
		UID:           "uid-1",
		Audiences:     []string{"kmaker"},
		Extra:         tokenReviewPodExtra("kernel-0", "pod-uid-0"),
	}}, fakePodGetter{pod: testKubernetesPod("cavalier", "kernel-0", "pod-uid-0", "kernel", "Running")})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*pb.MintKernelCertRequest)
	}{
		{
			name: "dns san",
			mutate: func(req *pb.MintKernelCertRequest) {
				req.CertRequest.DnsNames = []string{"kernel-0.cavalier.svc"}
			},
		},
		{
			name: "uri san",
			mutate: func(req *pb.MintKernelCertRequest) {
				req.CertRequest.UriSans = []string{"spiffe://local/dev/kernel/kernel-0"}
			},
		},
		{
			name: "ca request",
			mutate: func(req *pb.MintKernelCertRequest) {
				req.CertRequest.IsCa = true
				req.CertRequest.KeyUsage = x509pb.X509KeyUsage_X509_KEY_USAGE_CERT_SIGN
			},
		},
		{
			name: "non-kernel issuer",
			mutate: func(req *pb.MintKernelCertRequest) {
				req.CertRequest.Issuer = "oauth"
			},
		},
		{
			name: "unsafe ext key usage",
			mutate: func(req *pb.MintKernelCertRequest) {
				req.CertRequest.ExtKeyUsage = []x509pb.X509ExtKeyUsage{x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_CODE_SIGNING}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &pb.MintKernelCertRequest{
				K8SToken:    "token",
				CertRequest: testCertRequest(t, false),
			}
			tt.mutate(req)
			_, err := handler.MintKernelCert(context.Background(), nil, req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v", err)
			}
		})
	}
}

func TestHandlerAllowsCallerMetadataMatchingTokenBoundPod(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	handler, err := NewHandler(cfg, signer, fakeReviewer{status: &TokenReviewStatus{
		Authenticated: true,
		Username:      "system:serviceaccount:cavalier:kernel",
		UID:           "uid-1",
		Audiences:     []string{"kmaker"},
		Extra:         tokenReviewPodExtra("kernel-0", "pod-uid-0"),
	}}, fakePodGetter{pod: testKubernetesPod("cavalier", "kernel-0", "pod-uid-0", "kernel", "Running")})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	chain, err := handler.MintKernelCert(context.Background(), nil, &pb.MintKernelCertRequest{
		K8SToken:    "token",
		PodName:     "kernel-0",
		PodUid:      "pod-uid-0",
		InstanceId:  "kernel-0",
		CertRequest: testCertRequest(t, false),
	})
	if err != nil {
		t.Fatalf("MintKernelCert: %v", err)
	}
	leaf, err := parseCertProto(chain.GetChain()[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	assertURI(t, leaf.URIs, "spiffe://local/dev/kernel/kernel-0")
}

func TestHandlerRejectsCallerMetadataMismatchingTokenBoundPod(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	handler, err := NewHandler(cfg, signer, fakeReviewer{status: &TokenReviewStatus{
		Authenticated: true,
		Username:      "system:serviceaccount:cavalier:kernel",
		UID:           "uid-1",
		Audiences:     []string{"kmaker"},
		Extra:         tokenReviewPodExtra("kernel-0", "pod-uid-0"),
	}}, fakePodGetter{pod: testKubernetesPod("cavalier", "kernel-0", "pod-uid-0", "kernel", "Running")})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*pb.MintKernelCertRequest)
	}{
		{
			name: "instance id",
			mutate: func(req *pb.MintKernelCertRequest) {
				req.InstanceId = "kernel-1"
			},
		},
		{
			name: "pod name",
			mutate: func(req *pb.MintKernelCertRequest) {
				req.PodName = "kernel-1"
			},
		},
		{
			name: "pod uid",
			mutate: func(req *pb.MintKernelCertRequest) {
				req.PodUid = "pod-uid-1"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &pb.MintKernelCertRequest{
				K8SToken:    "token",
				CertRequest: testCertRequest(t, false),
			}
			tt.mutate(req)
			_, err := handler.MintKernelCert(context.Background(), nil, req)
			if status.Code(err) != codes.PermissionDenied {
				t.Fatalf("expected PermissionDenied, got %v", err)
			}
		})
	}
}

func TestHandlerRejectsMissingTokenBoundPod(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	handler, err := NewHandler(cfg, signer, fakeReviewer{status: &TokenReviewStatus{
		Authenticated: true,
		Username:      "system:serviceaccount:cavalier:kernel",
		UID:           "uid-1",
		Audiences:     []string{"kmaker"},
	}}, fakePodGetter{pod: testKubernetesPod("cavalier", "kernel-0", "pod-uid-0", "kernel", "Running")})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	_, err = handler.MintKernelCert(context.Background(), nil, &pb.MintKernelCertRequest{
		K8SToken:    "token",
		CertRequest: testCertRequest(t, false),
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestHandlerRejectsLivePodMismatch(t *testing.T) {
	tests := []struct {
		name string
		pod  *KubernetesPod
		err  error
	}{
		{
			name: "uid mismatch",
			pod:  testKubernetesPod("cavalier", "kernel-0", "pod-uid-1", "kernel", "Running"),
		},
		{
			name: "service account mismatch",
			pod:  testKubernetesPod("cavalier", "kernel-0", "pod-uid-0", "not-kernel", "Running"),
		},
		{
			name: "terminal pod",
			pod:  testKubernetesPod("cavalier", "kernel-0", "pod-uid-0", "kernel", "Succeeded"),
		},
		{
			name: "unknown pod phase",
			pod:  testKubernetesPod("cavalier", "kernel-0", "pod-uid-0", "kernel", "Unknown"),
		},
		{
			name: "pod lookup error",
			err:  context.DeadlineExceeded,
		},
		{
			name: "pod lookup returned nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			signer, err := NewSigner(cfg)
			if err != nil {
				t.Fatalf("NewSigner: %v", err)
			}
			handler, err := NewHandler(cfg, signer, fakeReviewer{status: &TokenReviewStatus{
				Authenticated: true,
				Username:      "system:serviceaccount:cavalier:kernel",
				UID:           "uid-1",
				Audiences:     []string{"kmaker"},
				Extra:         tokenReviewPodExtra("kernel-0", "pod-uid-0"),
			}}, fakePodGetter{pod: tt.pod, err: tt.err})
			if err != nil {
				t.Fatalf("NewHandler: %v", err)
			}

			_, err = handler.MintKernelCert(context.Background(), nil, &pb.MintKernelCertRequest{
				K8SToken:    "token",
				CertRequest: testCertRequest(t, false),
			})
			if status.Code(err) != codes.PermissionDenied {
				t.Fatalf("expected PermissionDenied, got %v", err)
			}
		})
	}
}

func TestHandlerRejectsWrongServiceAccount(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	handler, err := NewHandler(cfg, signer, fakeReviewer{status: &TokenReviewStatus{
		Authenticated: true,
		Username:      "system:serviceaccount:default:not-kernel",
		Audiences:     []string{"kmaker"},
	}}, fakePodGetter{pod: testKubernetesPod("default", "kernel-0", "pod-uid-0", "not-kernel", "Running")})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	_, err = handler.MintKernelCert(context.Background(), nil, &pb.MintKernelCertRequest{
		K8SToken:    "token",
		CertRequest: testCertRequest(t, false),
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestSignerRejectsOutOfScopeURISAN(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	req := &pb.MintKernelCertRequest{
		CertRequest: testCertRequest(t, false),
	}
	req.CertRequest.UriSans = []string{"spiffe://local/dev/oauth"}
	if _, err := signer.MintKernelCertificate(req); err == nil {
		t.Fatal("expected out-of-scope URI SAN to fail")
	}
}

func TestSignerRejectsKernelCA(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	req := &pb.MintKernelCertRequest{
		InstanceId:  "kernel-ca-0",
		CertRequest: testCertRequest(t, true),
	}
	if _, err := signer.MintKernelCertificate(req); err == nil {
		t.Fatal("expected CA request to fail")
	}
}

func testConfig(t *testing.T) *pb.Config {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	now := time.Now()
	tmpl := &stdx509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "kmaker-test-ca"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              stdx509.KeyUsageCertSign | stdx509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}
	der, err := stdx509.CreateCertificate(rand.Reader, tmpl, tmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	keyDER, err := stdx509.MarshalPKCS8PrivateKey(caKey)
	if err != nil {
		t.Fatalf("marshal CA key: %v", err)
	}
	cfg := &pb.Config{
		ClusterName:               "test",
		SpiffePrefix:              "local/dev",
		TokenAudience:             "kmaker",
		KernelCertLifetimeSeconds: 3600,
		KernelServiceAccounts:     []*pb.KernelServiceAccount{{Namespace: "cavalier", Name: "kernel"}},
		SigningCa: &x509pb.X509CertChain{
			Id: "test-ca",
			Key: &keypb.Key{
				Id:     "test-ca",
				Base64: encodePEM("PRIVATE KEY", keyDER),
			},
			Chain: []*x509pb.X509Cert{newCertProto("test-ca", der)},
		},
	}
	NormalizeConfig(cfg)
	return cfg
}

func testCertRequest(t *testing.T, isCA bool) *x509pb.X509CertRequest {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubDER, err := stdx509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	req := &x509pb.X509CertRequest{
		Key: &keypb.PublicKey{
			Id:     "kernel-key",
			Base64: encodePEM("PUBLIC KEY", pubDER),
		},
		Exp:    time.Now().Add(time.Hour).Unix(),
		Issuer: "kernel",
		IsCa:   isCA,
	}
	if isCA {
		req.KeyUsage = x509pb.X509KeyUsage_X509_KEY_USAGE_CERT_SIGN
		req.PathLenConstraint = 0
	} else {
		req.ExtKeyUsage = []x509pb.X509ExtKeyUsage{
			x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_SERVER_AUTH,
			x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_CLIENT_AUTH,
		}
	}
	return req
}

func encodePEM(typ string, der []byte) string {
	return base64.RawURLEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}))
}

func certPool(t *testing.T, cert *x509pb.X509Cert) *stdx509.CertPool {
	t.Helper()
	parsed, err := parseCertProto(cert)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}
	pool := stdx509.NewCertPool()
	pool.AddCert(parsed)
	return pool
}

func assertURI(t *testing.T, uris []*url.URL, want string) {
	t.Helper()
	for _, u := range uris {
		if u.String() == want {
			return
		}
	}
	t.Fatalf("URI SAN %q not found in %v", want, uris)
}

func tokenReviewPodExtra(podName, podUID string) map[string][]string {
	return map[string][]string{
		kubernetesPodNameExtraKey: []string{podName},
		kubernetesPodUIDExtraKey:  []string{podUID},
	}
}

func testKubernetesPod(namespace, name, uid, serviceAccountName, phase string) *KubernetesPod {
	return &KubernetesPod{
		Namespace:          namespace,
		Name:               name,
		UID:                uid,
		ServiceAccountName: serviceAccountName,
		Phase:              phase,
	}
}
