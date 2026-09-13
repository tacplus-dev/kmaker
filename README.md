# Kmaker

Kmaker is the bootstrap certificate authority for kernel instances.
Kernel pods present Kubernetes projected service account tokens to
`Kmaker.MintKernelCert`; kmaker validates the token with the Kubernetes
TokenReview API, verifies the token-bound pod with the Kubernetes Pod API, and
signs a Schematic `x509.X509CertRequest` under the operator-provisioned cluster
signing CA.

Kmaker intentionally starts outside the normal kernel-backed service manager:
kernel needs kmaker before kernel can mint ordinary Schematic service
certificates. The RPC still uses generated tacplus protobuf types.

## Configuration

Kmaker reads `--config` or `SERVICE_CONFIG`. Operator owns and renders this
config.

| Field | Default | Description |
|-------|---------|-------------|
| `cluster_name` | - | Logical Schematic cluster name |
| `listener` | `0.0.0.0:8443` | gRPC bind address |
| `signing_ca` | - | Cluster signing CA chain and private key |
| `spiffe_prefix` | `local.dev` | SPIFFE trust domain (URI authority, no `/`) for `spiffe://<prefix>/kernel` |
| `token_audience` | `kmaker` | Required Kubernetes token audience |
| `kernel_service_accounts` | `schematic/kernel` | Service accounts allowed to request kernel certs |
| `kernel_cert_lifetime_seconds` | `86400` | Maximum issued cert lifetime |
| `tls` | - | Optional serving certificate for kmaker |

When `tls` is unset, kmaker serves plaintext gRPC and logs a warning. That mode
is intended only for local/development bootstrap.

## Kubernetes

The kmaker pod must be allowed to create TokenReview requests and read Pods in
the Schematic namespace:

```yaml
rules:
  - apiGroups: ["authentication.k8s.io"]
    resources: ["tokenreviews"]
    verbs: ["create"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get"]
```

Kmaker derives the issued kernel instance identity from TokenReview pod metadata
and live Pod facts. Caller-supplied SANs, CA bits, key usages, and mismatching
pod/instance metadata are rejected.

## Build

```bash
tacplus build --src ../../proto --tag latest
go test ./...
go build ./...
```
