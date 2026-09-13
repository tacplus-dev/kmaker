# kmaker low-priority security-audit fixes (2026-09-12)

Source: `decisions/security-audit-2026-09-12/CONSOLIDATED_REPORT.md`. The audit found the
core cert-minting path sound; these are the four flagged secondary items.

## What changed

1. **Startup CA sanity check** — `NewSigner` now calls `validateSigningCA`
   (`internal/kmaker/signer.go`): rejects a `signing_ca` chain[0] that is not a CA
   (`BasicConstraintsValid && IsCA`) and a private key whose public half does not equal the
   certificate's public key (`crypto.PublicKey.Equal`, which rsa/ecdsa/ed25519 all implement —
   no signing round-trip needed). Both fail startup with a named error.
2. **`spiffe_prefix` validation** — `normalizeSpiffePrefix` now returns an error and delegates to
   the generated chassis `spiffe.NormalizeTrustDomain`, so a prefix containing `/` (or a query,
   fragment, backslash, control char, or encoded separator) fails startup instead of silently
   minting under a different trust domain than the operator intended. This matches the proto,
   which documents `spiffe_prefix` as the URI authority with default `local.dev`.
   The README table said `local/dev`; that value now refuses to start, so the README was corrected.
3. **Instance SPIFFE ID carries namespace and pod UID** — the issued per-instance URI SAN went
   from `spiffe://<td>/kernel/<podName>` to `spiffe://<td>/kernel/<namespace>/<podName>/<podUID>`.
   The collective `spiffe://<td>/kernel` SAN is unchanged.
4. **Weak-key floor** — `validateSubjectPublicKey` rejects RSA subject keys under 2048 bits and
   EC keys on any curve other than P-256/P-384/P-521; Ed25519 is accepted; anything else is
   rejected as unsupported.

Tests: `internal/kmaker/signer_hardening_test.go`. Each was verified red with its fix reverted.

## Decisions taken (no one to ask mid-task)

- **Weak-key threshold: RSA >= 2048 bits, EC in {P-256, P-384, P-521}.** Not invented: the
  platform key chassis (`model/chassis/key/key.go`, generated) already rejects an RSA key whose
  modulus is under 256 bytes (= 2048 bits) and only maps ES256/ES384/ES512 curves. kmaker parses
  subject keys with raw `crypto/x509` rather than that chassis, so the floor is restated in
  `minSubjectRSAKeyBits` rather than reached through a wrapper the mint path does not use.
  *Open question:* should the mint path parse subject keys through `key.NewImmutablePublicKey`
  instead, and drop the local constant? That is a larger refactor of the parse path than this
  cleanup warranted. Default taken: local constant, documented as mirroring the chassis.

- **Instance SPIFFE ID shape: `spiffe://<td>/kernel/<namespace>/<podName>/<podUID>`.** The pod
  UID alone would remove the ambiguity the audit describes (pod recycle, same pod name in two
  namespaces); the namespace was included as well because the audit names both and because the
  namespace is the authorization-relevant scope. No consumer parses this SAN today — a repo-wide
  grep for `MintKernelCert` finds only kmaker itself and planning docs
  (`platform/todo/KERNEL_KMAKER_K8S_IDENTITY_PLAN.md`, `platform/services/kernel/IDENTITY.md`),
  so the shape change breaks nothing in tree. *Open question:* kmaker's whole `/kernel/...` shape
  is off-grammar for the platform — the canonical kernel service identity built by operator is
  `spiffe://<td>/system/services/kernel` (`operator/pkg/bootstrap/deploycrypto.go`,
  `KernelServiceSpiffeURI`), and a workload child would be `.../workloads/<name>` per
  `model/chassis/spiffe`. Aligning kmaker to that grammar is a real change with a real consumer
  story (kernel's bootstrap) and was out of scope here. Default taken: keep kmaker's existing
  `/kernel` shape, only disambiguate the instance segment.

- **`podNamespace` is a new parameter on `Signer.MintKernelCertificate`.** `MintKernelCertRequest`
  has no namespace field and the proto is tacplus-generated, so the verified namespace is passed
  as an argument from the handler (which has it from the TokenReview username and the live Pod
  cross-check) rather than by editing generated code.

## Not addressed (flagged by the audit, not in this task's scope)

Plaintext Kubernetes API override / missing-CA-file trust widening; kernel SA allowlist
fail-open in `AllowedServiceAccounts`; no client mTLS on the serving port; signer failures
collapsing to `InvalidArgument`; Kubernetes error bodies echoed to callers; the dead
`requested` SAN parameter and the `mapKeyUsage` unknown-enum fallback; defaults declared in
three places; the README plaintext-fallback drift.
