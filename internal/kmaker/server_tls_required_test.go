package kmaker

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	stdx509 "crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"testing"
	"time"

	pb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/services/kmaker"
	keypb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/sys/key"
	x509pb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/sys/x509"
)

// servingTLSChain builds a self-signed leaf usable as kmaker's serving credential.
func servingTLSChain(t *testing.T) *x509pb.X509CertChain {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate serving key: %v", err)
	}
	now := time.Now()
	tmpl := &stdx509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "kmaker"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              stdx509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := stdx509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create serving cert: %v", err)
	}
	keyDER, err := stdx509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal serving key: %v", err)
	}
	return &x509pb.X509CertChain{
		Id:    "kmaker-serving",
		Key:   &keypb.Key{Id: "kmaker-serving", Base64: encodePEM("PRIVATE KEY", keyDER)},
		Chain: []*x509pb.X509Cert{newCertProto("kmaker-serving", der)},
	}
}

// A config regression that drops the tls section must STOP kmaker, not
// downgrade it to plaintext with a log line nobody reads.
func TestServingTLSAbsentWithoutOptInIsAnError(t *testing.T) {
	t.Setenv(AllowPlaintextEnv, "")

	opts, err := ServingTLSOptions(&pb.Config{})
	if err == nil {
		t.Fatalf("expected refusal with no TLS and no opt-in, got %d server options and nil error", len(opts))
	}
	if !errors.Is(err, ErrTLSRequired) {
		t.Fatalf("expected ErrTLSRequired, got %v", err)
	}
}

// Values other than "true" are not an opt-in — a half-set variable must not
// open the plaintext path.
func TestServingTLSAbsentRejectsNonTrueOptIn(t *testing.T) {
	for _, v := range []string{"false", "0", "1", "yes", "TRUE-ish"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv(AllowPlaintextEnv, v)
			if _, err := ServingTLSOptions(&pb.Config{}); !errors.Is(err, ErrTLSRequired) {
				t.Fatalf("%s=%q must not permit plaintext, got err=%v", AllowPlaintextEnv, v, err)
			}
		})
	}
}

// The dev bootstrap workflow stays regression-free: explicit opt-in still starts.
func TestServingTLSAbsentWithOptInStarts(t *testing.T) {
	t.Setenv(AllowPlaintextEnv, "true")

	opts, err := ServingTLSOptions(&pb.Config{})
	if err != nil {
		t.Fatalf("expected plaintext start with %s=true, got %v", AllowPlaintextEnv, err)
	}
	if len(opts) != 0 {
		t.Fatalf("expected no transport credentials in plaintext mode, got %d options", len(opts))
	}
}

// TLS present behaves exactly as before, opt-in or not.
func TestServingTLSPresentStarts(t *testing.T) {
	for _, optIn := range []string{"", "true"} {
		t.Run("optin="+optIn, func(t *testing.T) {
			t.Setenv(AllowPlaintextEnv, optIn)

			cfg := &pb.Config{Tls: servingTLSChain(t)}
			opts, err := ServingTLSOptions(cfg)
			if err != nil {
				t.Fatalf("expected TLS start, got %v", err)
			}
			if len(opts) != 1 {
				t.Fatalf("expected exactly one credentials option, got %d", len(opts))
			}
		})
	}
}
