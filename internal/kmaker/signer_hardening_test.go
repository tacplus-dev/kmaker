package kmaker

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	stdx509 "crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strings"
	"testing"
	"time"

	keypb "gitlab.com/seabeam/schematic/platform/services/kmaker/model/types/sys/key"
	x509pb "gitlab.com/seabeam/schematic/platform/services/kmaker/model/types/sys/x509"
	pb "gitlab.com/seabeam/schematic/platform/services/kmaker/model/types/services/kmaker"
)

// signingCAInto replaces cfg.SigningCa with a certificate built from tmpl and
// signed by certKey, paired with keyForConfig as the configured private key.
func signingCAInto(t *testing.T, cfg *pb.Config, tmpl *stdx509.Certificate, certKey, keyForConfig *ecdsa.PrivateKey) {
	t.Helper()
	der, err := stdx509.CreateCertificate(rand.Reader, tmpl, tmpl, &certKey.PublicKey, certKey)
	if err != nil {
		t.Fatalf("create signing CA cert: %v", err)
	}
	keyDER, err := stdx509.MarshalPKCS8PrivateKey(keyForConfig)
	if err != nil {
		t.Fatalf("marshal signing CA key: %v", err)
	}
	cfg.SigningCa = &x509pb.X509CertChain{
		Id:    "test-ca",
		Key:   &keypb.Key{Id: "test-ca", Base64: encodePEM("PRIVATE KEY", keyDER)},
		Chain: []*x509pb.X509Cert{newCertProto("test-ca", der)},
	}
}

func caTemplate(isCA bool) *stdx509.Certificate {
	now := time.Now()
	tmpl := &stdx509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "kmaker-test-ca"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(24 * time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	if isCA {
		tmpl.KeyUsage = stdx509.KeyUsageCertSign | stdx509.KeyUsageCRLSign
		tmpl.MaxPathLen = 1
	} else {
		tmpl.KeyUsage = stdx509.KeyUsageDigitalSignature
	}
	return tmpl
}

func TestNewSignerRejectsNonCASigningCertificate(t *testing.T) {
	cfg := testConfig(t)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signingCAInto(t, cfg, caTemplate(false), key, key)

	_, err = NewSigner(cfg)
	if err == nil {
		t.Fatal("expected startup to fail for a non-CA signing certificate")
	}
	if !strings.Contains(err.Error(), "not a CA certificate") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewSignerRejectsSigningKeyThatDoesNotMatchCertificate(t *testing.T) {
	cfg := testConfig(t)
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate cert key: %v", err)
	}
	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}
	signingCAInto(t, cfg, caTemplate(true), certKey, otherKey)

	_, err = NewSigner(cfg)
	if err == nil {
		t.Fatal("expected startup to fail when the signing key does not match the certificate")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewSignerAcceptsMatchingCASigningMaterial(t *testing.T) {
	if _, err := NewSigner(testConfig(t)); err != nil {
		t.Fatalf("NewSigner with a valid CA: %v", err)
	}
}

func TestNewSignerRejectsSpiffePrefixWithPathSeparator(t *testing.T) {
	cfg := testConfig(t)
	cfg.SpiffePrefix = "local/dev"
	_, err := NewSigner(cfg)
	if err == nil {
		t.Fatal("expected a spiffe_prefix containing '/' to be rejected")
	}
	if !strings.Contains(err.Error(), "spiffe_prefix") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeSpiffePrefix(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "local.dev", want: "local.dev"},
		{in: "  local.dev  ", want: "local.dev"},
		{in: "spiffe://local.dev", want: "local.dev"},
		{in: "", want: defaultSpiffePrefix},
		{in: "local/dev", wantErr: true},
		{in: "local.dev/", wantErr: true},
		{in: "local.dev#x", wantErr: true},
	}
	for _, tt := range tests {
		got, err := normalizeSpiffePrefix(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("normalizeSpiffePrefix(%q) = %q, want error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("normalizeSpiffePrefix(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("normalizeSpiffePrefix(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIssuedInstanceSpiffeIDCarriesNamespaceAndPodUID(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	handler, err := NewHandler(cfg, signer, fakeReviewer{status: &TokenReviewStatus{
		Authenticated: true,
		Username:      "system:serviceaccount:schematic:kernel",
		UID:           "uid-1",
		Audiences:     []string{"kmaker"},
		Extra:         tokenReviewPodExtra("kernel-0", "pod-uid-0"),
	}}, fakePodGetter{pod: testKubernetesPod("schematic", "kernel-0", "pod-uid-0", "kernel", "Running")})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	chain, err := handler.MintKernelCert(context.Background(), nil, &pb.MintKernelCertRequest{
		K8SToken:    "token",
		CertRequest: testCertRequest(t, false),
	})
	if err != nil {
		t.Fatalf("MintKernelCert: %v", err)
	}
	leaf, err := parseCertProto(chain.GetChain()[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	assertURI(t, leaf.URIs, "spiffe://local.dev/kernel/schematic/kernel-0/pod-uid-0")
	// The pod-name-only identity must not be issued: a recycled pod or a
	// same-named pod in another namespace would otherwise share it.
	for _, u := range leaf.URIs {
		if u.String() == "spiffe://local.dev/kernel/kernel-0" {
			t.Fatalf("issued ambiguous pod-name-only SPIFFE ID: %v", leaf.URIs)
		}
	}
}

func TestMintRejectsWeakSubjectKeys(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	weakRSA, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate weak RSA key: %v", err)
	}
	weakEC, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-224 key: %v", err)
	}

	tests := []struct {
		name string
		pub  crypto.PublicKey
		want string
	}{
		{name: "1024-bit RSA", pub: &weakRSA.PublicKey, want: "below the 2048-bit minimum"},
		{name: "P-224 EC", pub: &weakEC.PublicKey, want: "unsupported curve"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mintRequestForPublicKey(t, tt.pub)
			_, err := signer.MintKernelCertificate(req, "schematic")
			if err == nil {
				t.Fatalf("expected %s subject key to be rejected", tt.name)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestMintAcceptsStrongSubjectKeys(t *testing.T) {
	cfg := testConfig(t)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	strongRSA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	strongEC, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}

	for name, pub := range map[string]crypto.PublicKey{
		"2048-bit RSA": &strongRSA.PublicKey,
		"P-256 EC":     &strongEC.PublicKey,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := signer.MintKernelCertificate(mintRequestForPublicKey(t, pub), "schematic"); err != nil {
				t.Fatalf("expected %s subject key to be accepted: %v", name, err)
			}
		})
	}
}

func mintRequestForPublicKey(t *testing.T, pub crypto.PublicKey) *pb.MintKernelCertRequest {
	t.Helper()
	pubDER, err := stdx509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return &pb.MintKernelCertRequest{
		PodName:    "kernel-0",
		PodUid:     "pod-uid-0",
		InstanceId: "kernel-0",
		CertRequest: &x509pb.X509CertRequest{
			Key:    &keypb.PublicKey{Id: "kernel-key", Base64: encodePEM("PUBLIC KEY", pubDER)},
			Exp:    time.Now().Add(time.Hour).Unix(),
			Issuer: "kernel",
			ExtKeyUsage: []x509pb.X509ExtKeyUsage{
				x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_SERVER_AUTH,
				x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_CLIENT_AUTH,
			},
		},
	}
}
