package kmaker

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	stdx509 "crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"gitlab.com/seabeam/schematic/platform/services/kmaker/model/chassis/spiffe"
	keypb "gitlab.com/seabeam/schematic/platform/services/kmaker/model/types/sys/key"
	x509pb "gitlab.com/seabeam/schematic/platform/services/kmaker/model/types/sys/x509"
	pb "gitlab.com/seabeam/schematic/platform/services/kmaker/model/types/services/kmaker"
)

// minSubjectRSAKeyBits is the smallest RSA subject key kmaker will sign. It
// matches the floor the platform key chassis already enforces, which rejects an
// RSA key whose modulus is under 256 bytes (model/chassis/key/key.go).
const minSubjectRSAKeyBits = 2048

type Signer struct {
	caCert       *stdx509.Certificate
	caKey        crypto.Signer
	caChain      []*x509pb.X509Cert
	spiffePrefix string
	certLifetime time.Duration
}

func NewSigner(cfg *pb.Config) (*Signer, error) {
	if cfg.GetSigningCa() == nil {
		return nil, fmt.Errorf("signing_ca is required")
	}
	if len(cfg.GetSigningCa().GetChain()) == 0 {
		return nil, fmt.Errorf("signing_ca.chain is required")
	}
	if cfg.GetSigningCa().GetKey() == nil {
		return nil, fmt.Errorf("signing_ca.key is required")
	}
	caCert, err := parseCertProto(cfg.GetSigningCa().GetChain()[0])
	if err != nil {
		return nil, fmt.Errorf("parse signing_ca certificate: %w", err)
	}
	caKey, err := parsePrivateKeyProto(cfg.GetSigningCa().GetKey())
	if err != nil {
		return nil, fmt.Errorf("parse signing_ca key: %w", err)
	}
	if err := validateSigningCA(caCert, caKey); err != nil {
		return nil, err
	}

	spiffePrefix, err := normalizeSpiffePrefix(cfg.GetSpiffePrefix())
	if err != nil {
		return nil, fmt.Errorf("invalid spiffe_prefix: %w", err)
	}

	lifetime := time.Duration(cfg.GetKernelCertLifetimeSeconds()) * time.Second
	if lifetime <= 0 {
		lifetime = time.Duration(defaultCertLifetime) * time.Second
	}

	return &Signer{
		caCert:       caCert,
		caKey:        caKey,
		caChain:      cfg.GetSigningCa().GetChain(),
		spiffePrefix: spiffePrefix,
		certLifetime: lifetime,
	}, nil
}

func (s *Signer) MintKernelCertificate(req *pb.MintKernelCertRequest, podNamespace string) (*x509pb.X509CertChain, error) {
	if req.GetCertRequest() == nil {
		return nil, fmt.Errorf("cert_request is required")
	}
	certReq := req.GetCertRequest()
	pub, err := parsePublicKeyProto(certReq.GetKey())
	if err != nil {
		return nil, err
	}
	if err := validateSubjectPublicKey(pub); err != nil {
		return nil, err
	}

	if certReq.GetIsCa() {
		return nil, fmt.Errorf("kernel certificates may not request CA issuance")
	}
	if certReq.GetPathLenConstraint() != 0 {
		return nil, fmt.Errorf("kernel leaf certificates may not set path_len_constraint")
	}
	if certReq.GetIssuer() != "" && certReq.GetIssuer() != "kernel" {
		return nil, fmt.Errorf("kernel certificate subject is server controlled")
	}
	if certReq.GetKeyUsage() != x509pb.X509KeyUsage_X509_KEY_USAGE_DIGITAL_SIGNATURE {
		return nil, fmt.Errorf("kernel certificate key usage is server controlled")
	}
	for _, usage := range certReq.GetExtKeyUsage() {
		if usage != x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_SERVER_AUTH &&
			usage != x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_CLIENT_AUTH {
			return nil, fmt.Errorf("kernel certificate extended key usage is server controlled")
		}
	}
	if len(certReq.GetDnsNames()) > 0 {
		return nil, fmt.Errorf("kernel certificate DNS SANs are server controlled")
	}
	if len(certReq.GetUriSans()) > 0 {
		return nil, fmt.Errorf("kernel certificate URI SANs are server controlled")
	}
	if req.GetInstanceId() == "" {
		return nil, fmt.Errorf("kernel certificate instance_id must be derived before signing")
	}
	if podNamespace == "" {
		return nil, fmt.Errorf("kernel certificate pod namespace must be derived before signing")
	}
	if req.GetPodUid() == "" {
		return nil, fmt.Errorf("kernel certificate pod_uid must be derived before signing")
	}

	uriSANs, err := s.kernelURISANs(nil, podNamespace, req.GetInstanceId(), req.GetPodUid())
	if err != nil {
		return nil, err
	}

	now := time.Now()
	notAfter, err := s.notAfter(now, certReq.GetExp())
	if err != nil {
		return nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	template := &stdx509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "kernel"},
		NotBefore:             now.Add(-1 * time.Minute),
		NotAfter:              notAfter,
		URIs:                  uriSANs,
		BasicConstraintsValid: true,
		IsCA:                  false,
	}
	template.KeyUsage = stdx509.KeyUsageDigitalSignature
	template.ExtKeyUsage = []stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth, stdx509.ExtKeyUsageClientAuth}

	der, err := stdx509.CreateCertificate(rand.Reader, template, s.caCert, pub, s.caKey)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	id := certReq.GetKey().GetId()
	if req.GetInstanceId() != "" {
		id = req.GetInstanceId()
	}
	if id == "" {
		id = "kernel"
	}

	out := &x509pb.X509CertChain{
		Id:    id,
		Chain: make([]*x509pb.X509Cert, 0, 1+len(s.caChain)),
	}
	out.Chain = append(out.Chain, newCertProto(id, der))
	out.Chain = append(out.Chain, s.caChain...)
	return out, nil
}

// kernelURISANs builds the issued URI SANs: the collective kernel identity plus
// the per-instance identity. The instance path carries the Kubernetes namespace
// and the pod UID alongside the pod name so that a recycled pod (same name, new
// UID) and two same-named pods in different namespaces never share one instance
// identity for the lifetime of a certificate.
func (s *Signer) kernelURISANs(requested []string, podNamespace, instanceID, podUID string) ([]*url.URL, error) {
	base := s.kernelBaseURI()
	seen := map[string]struct{}{base: {}}
	for _, raw := range requested {
		if raw == "" {
			continue
		}
		if raw != base && !strings.HasPrefix(raw, base+"/") {
			return nil, fmt.Errorf("uri SAN %q is outside kernel identity %q", raw, base)
		}
		seen[raw] = struct{}{}
	}
	if instanceID != "" {
		for _, segment := range []string{podNamespace, instanceID, podUID} {
			if !safePathSegment(segment) {
				return nil, fmt.Errorf("kernel instance segment %q is not a safe SPIFFE path segment", segment)
			}
		}
		seen[base+"/"+podNamespace+"/"+instanceID+"/"+podUID] = struct{}{}
	}

	out := make([]*url.URL, 0, len(seen))
	for raw := range seen {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "spiffe" || u.Host == "" {
			return nil, fmt.Errorf("invalid SPIFFE URI %q", raw)
		}
		out = append(out, u)
	}
	return out, nil
}

func (s *Signer) kernelBaseURI() string {
	return "spiffe://" + s.spiffePrefix + "/kernel"
}

// validateSigningCA fails startup on a misconfigured signing CA: a certificate
// that is not a CA, or a private key that does not belong to it. Without this
// kmaker starts healthy and then either fails every mint or issues leaves that
// fail chain verification at every peer.
func validateSigningCA(caCert *stdx509.Certificate, caKey crypto.Signer) error {
	if !caCert.BasicConstraintsValid || !caCert.IsCA {
		return fmt.Errorf("signing_ca certificate %q is not a CA certificate", caCert.Subject.CommonName)
	}
	type equaler interface {
		Equal(crypto.PublicKey) bool
	}
	certPub, ok := caCert.PublicKey.(equaler)
	if !ok {
		return fmt.Errorf("signing_ca certificate %q has an unsupported public key type %T", caCert.Subject.CommonName, caCert.PublicKey)
	}
	if !certPub.Equal(caKey.Public()) {
		return fmt.Errorf("signing_ca private key does not match the signing_ca certificate %q", caCert.Subject.CommonName)
	}
	return nil
}

// validateSubjectPublicKey enforces a minimum subject key strength. Every issued
// certificate carries the shared kernel SAN, so a weak subject key would hold
// the collective kernel identity, not just one pod's.
func validateSubjectPublicKey(pub any) error {
	switch key := pub.(type) {
	case *rsa.PublicKey:
		if bits := key.N.BitLen(); bits < minSubjectRSAKeyBits {
			return fmt.Errorf("subject RSA key of %d bits is below the %d-bit minimum", bits, minSubjectRSAKeyBits)
		}
	case *ecdsa.PublicKey:
		switch key.Curve {
		case elliptic.P256(), elliptic.P384(), elliptic.P521():
		default:
			return fmt.Errorf("subject EC key uses unsupported curve %q", key.Curve.Params().Name)
		}
	case ed25519.PublicKey:
	default:
		return fmt.Errorf("subject key type %T is not supported", pub)
	}
	return nil
}

func (s *Signer) notAfter(now time.Time, requestedExp int64) (time.Time, error) {
	notAfter := now.Add(s.certLifetime)
	if requestedExp > 0 {
		requested := time.Unix(requestedExp, 0)
		if !requested.After(now) {
			return time.Time{}, fmt.Errorf("requested certificate expiry is in the past")
		}
		if requested.Before(notAfter) {
			notAfter = requested
		}
	}
	if s.caCert.NotAfter.Before(notAfter) {
		notAfter = s.caCert.NotAfter
	}
	if !notAfter.After(now) {
		return time.Time{}, fmt.Errorf("signing CA is expired")
	}
	return notAfter, nil
}

func TLSCertificateFromChain(chain *x509pb.X509CertChain) (tls.Certificate, error) {
	if chain == nil || len(chain.GetChain()) == 0 || chain.GetKey() == nil {
		return tls.Certificate{}, fmt.Errorf("tls certificate chain and key are required")
	}
	var certPEM []byte
	for _, cert := range chain.GetChain() {
		block, err := certPEMBlock(cert)
		if err != nil {
			return tls.Certificate{}, err
		}
		certPEM = append(certPEM, block...)
	}
	keyPEM, err := keyPEMBlock(chain.GetKey())
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

func parseCertProto(cert *x509pb.X509Cert) (*stdx509.Certificate, error) {
	if cert == nil {
		return nil, fmt.Errorf("certificate is nil")
	}
	data, err := decodeBase64(cert.GetBase64())
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block != nil {
		data = block.Bytes
	}
	return stdx509.ParseCertificate(data)
}

func parsePrivateKeyProto(k *keypb.Key) (crypto.Signer, error) {
	if k == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	data, err := decodeBase64(k.GetBase64())
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block != nil {
		data = block.Bytes
	}
	parsed, err := stdx509.ParsePKCS8PrivateKey(data)
	if err != nil {
		if key, ecErr := stdx509.ParseECPrivateKey(data); ecErr == nil {
			parsed = key
		} else if key, rsaErr := stdx509.ParsePKCS1PrivateKey(data); rsaErr == nil {
			parsed = key
		} else {
			return nil, err
		}
	}
	signer, ok := parsed.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("private key does not implement crypto.Signer")
	}
	return signer, nil
}

func parsePublicKeyProto(k *keypb.PublicKey) (any, error) {
	if k == nil {
		return nil, fmt.Errorf("cert_request.key is required")
	}
	data, err := decodeBase64(k.GetBase64())
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block != nil {
		data = block.Bytes
	}
	pub, err := stdx509.ParsePKIXPublicKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	return pub, nil
}

func certPEMBlock(cert *x509pb.X509Cert) ([]byte, error) {
	data, err := decodeBase64(cert.GetBase64())
	if err != nil {
		return nil, fmt.Errorf("decode certificate %q: %w", cert.GetId(), err)
	}
	if block, _ := pem.Decode(data); block != nil {
		return pem.EncodeToMemory(block), nil
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: data}), nil
}

func keyPEMBlock(key *keypb.Key) ([]byte, error) {
	data, err := decodeBase64(key.GetBase64())
	if err != nil {
		return nil, fmt.Errorf("decode private key %q: %w", key.GetId(), err)
	}
	if block, _ := pem.Decode(data); block != nil {
		return pem.EncodeToMemory(block), nil
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: data}), nil
}

func newCertProto(id string, der []byte) *x509pb.X509Cert {
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return &x509pb.X509Cert{
		Id:     id,
		Base64: base64.RawURLEncoding.EncodeToString(pemBytes),
	}
}

func decodeBase64(value string) ([]byte, error) {
	if value == "" {
		return nil, fmt.Errorf("base64 value is empty")
	}
	if data, err := base64.RawURLEncoding.DecodeString(value); err == nil {
		return data, nil
	}
	if data, err := base64.URLEncoding.DecodeString(value); err == nil {
		return data, nil
	}
	if data, err := base64.StdEncoding.DecodeString(value); err == nil {
		return data, nil
	}
	return nil, fmt.Errorf("invalid base64 encoding")
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, err
	}
	if serial.Sign() == 0 {
		serial = big.NewInt(1)
	}
	return serial, nil
}

func validateDNSNames(names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("dns_names contains an empty value")
		}
		if len(name) > 253 {
			return nil, fmt.Errorf("dns_name %q exceeds 253 characters", name)
		}
		out = append(out, name)
	}
	return out, nil
}

func safePathSegment(v string) bool {
	if v == "" || strings.Contains(v, "/") || v == "." || v == ".." {
		return false
	}
	for _, r := range v {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '.', '_', '-', ':':
			continue
		default:
			return false
		}
	}
	return true
}

// normalizeSpiffePrefix validates the configured prefix as a SPIFFE trust
// domain. The proto documents it as the URI authority, so a value carrying a
// path separator ("local/dev") is rejected rather than silently minting under a
// different trust domain than the operator intended.
func normalizeSpiffePrefix(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	prefix = strings.TrimPrefix(prefix, "spiffe://")
	if prefix == "" {
		prefix = defaultSpiffePrefix
	}
	return spiffe.NormalizeTrustDomain(prefix)
}

func mapKeyUsage(v x509pb.X509KeyUsage) stdx509.KeyUsage {
	switch v {
	case x509pb.X509KeyUsage_X509_KEY_USAGE_CONTENT_COMMITMENT:
		return stdx509.KeyUsageContentCommitment
	case x509pb.X509KeyUsage_X509_KEY_USAGE_KEY_ENCIPHERMENT:
		return stdx509.KeyUsageKeyEncipherment
	case x509pb.X509KeyUsage_X509_KEY_USAGE_DATA_ENCIPHERMENT:
		return stdx509.KeyUsageDataEncipherment
	case x509pb.X509KeyUsage_X509_KEY_USAGE_KEY_AGREEMENT:
		return stdx509.KeyUsageKeyAgreement
	case x509pb.X509KeyUsage_X509_KEY_USAGE_CERT_SIGN:
		return stdx509.KeyUsageCertSign
	case x509pb.X509KeyUsage_X509_KEY_USAGE_CRL_SIGN:
		return stdx509.KeyUsageCRLSign
	case x509pb.X509KeyUsage_X509_KEY_USAGE_ENCIPHER_ONLY:
		return stdx509.KeyUsageEncipherOnly
	case x509pb.X509KeyUsage_X509_KEY_USAGE_DECIPHER_ONLY:
		return stdx509.KeyUsageDecipherOnly
	default:
		return stdx509.KeyUsageDigitalSignature
	}
}

func mapExtKeyUsages(values []x509pb.X509ExtKeyUsage) []stdx509.ExtKeyUsage {
	out := make([]stdx509.ExtKeyUsage, 0, len(values))
	for _, v := range values {
		switch v {
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_ANY:
			out = append(out, stdx509.ExtKeyUsageAny)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_SERVER_AUTH:
			out = append(out, stdx509.ExtKeyUsageServerAuth)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_CLIENT_AUTH:
			out = append(out, stdx509.ExtKeyUsageClientAuth)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_CODE_SIGNING:
			out = append(out, stdx509.ExtKeyUsageCodeSigning)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_EMAIL_PROTECTION:
			out = append(out, stdx509.ExtKeyUsageEmailProtection)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_IPSEC_END_SYSTEM:
			out = append(out, stdx509.ExtKeyUsageIPSECEndSystem)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_IPSEC_TUNNEL:
			out = append(out, stdx509.ExtKeyUsageIPSECTunnel)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_IPSEC_USER:
			out = append(out, stdx509.ExtKeyUsageIPSECUser)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_TIME_STAMPING:
			out = append(out, stdx509.ExtKeyUsageTimeStamping)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_OCSP_SIGNING:
			out = append(out, stdx509.ExtKeyUsageOCSPSigning)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_MICROSOFT_SERVER_GATED_CRYPTO:
			out = append(out, stdx509.ExtKeyUsageMicrosoftServerGatedCrypto)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_NETSCAPE_SERVER_GATED_CRYPTO:
			out = append(out, stdx509.ExtKeyUsageNetscapeServerGatedCrypto)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_MICROSOFT_COMMERCIAL_CODE_SIGNING:
			out = append(out, stdx509.ExtKeyUsageMicrosoftCommercialCodeSigning)
		case x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_MICROSOFT_KERNEL_CODE_SIGNING:
			out = append(out, stdx509.ExtKeyUsageMicrosoftKernelCodeSigning)
		}
	}
	return out
}
