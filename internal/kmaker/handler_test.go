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

	keypb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/libs/key"
	x509pb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/libs/x509"
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
	}})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := &pb.SignKernelCertRequest{
		K8SToken:    "token",
		InstanceId:  "kernel-0",
		CertRequest: testCertRequest(t, false),
	}
	chain, err := handler.SignKernelCert(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("SignKernelCert: %v", err)
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
	}})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	_, err = handler.SignKernelCert(context.Background(), nil, &pb.SignKernelCertRequest{
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
	req := &pb.SignKernelCertRequest{
		CertRequest: testCertRequest(t, false),
	}
	req.CertRequest.UriSans = []string{"spiffe://local/dev/oauth"}
	if _, err := signer.SignKernelCertificate(req); err == nil {
		t.Fatal("expected out-of-scope URI SAN to fail")
	}
}

func TestSignerAllowsKernelPathLenZeroCA(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	req := &pb.SignKernelCertRequest{
		InstanceId:  "kernel-ca-0",
		CertRequest: testCertRequest(t, true),
	}
	chain, err := signer.SignKernelCertificate(req)
	if err != nil {
		t.Fatalf("SignKernelCertificate: %v", err)
	}
	cert, err := parseCertProto(chain.GetChain()[0])
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	if !cert.IsCA {
		t.Fatal("expected CA certificate")
	}
	if cert.MaxPathLen != 0 || !cert.MaxPathLenZero {
		t.Fatalf("expected pathlen zero CA, got MaxPathLen=%d MaxPathLenZero=%t", cert.MaxPathLen, cert.MaxPathLenZero)
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
