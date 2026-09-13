# kmaker Security & Quality Audit — Consolidated Report
2026-09-12 — multimodal review: Fable 5.1 (Claude), Codex CLI (GPT), Grok 4.6 (via GitHub Copilot CLI)
Scope: `internal/` (10 Go files). `model/` (tacplus-generated) and `bin/` excluded.

**Good news, unanimous across all three tools: the core cert-minting path is sound.** kmaker holds
the cluster's CA signing key and mints certificates for other services — the single highest-value
target audited so far given a compromise here forges trust for the entire mesh. Fable's words:
"the identity-binding path is sound... I found no way for a caller to obtain a certificate for an
identity that is not theirs." Codex, independently: "no caller-identity binding, SAN/CN injection,
CA-issuance, or caller-controlled validity bypass in the mint path... signed subject, SANs, usages,
CA bit, and maximum expiry are server-controlled." Grok, independently: "Mint path is recently
hardened." Identity comes from Kubernetes TokenReview extras cross-checked against a live Pod GET,
caller-supplied SANs/CA-bits/key-usages/validity are discarded and rebuilt server-side, and the CA
private key itself is never returned to any caller. This is the cleanest core-security-logic result
of the seven services audited today.

---

## Real findings, all secondary to the sound core path

### Medium
- **Signing CA is never sanity-checked at startup** [VERIFIED] — `internal/kmaker/signer.go:30-47`.
  I read the constructor directly: the CA certificate and its private key are parsed independently,
  with no check that the certificate is actually a CA (`IsCA`) and no check that the key matches
  the certificate's public key. A config error (wrong key paired with the cert, or a leaf
  certificate placed where the CA chain is expected) would pass startup silently, then either fail
  every mint or issue certificates that fail chain verification everywhere — with kmaker reporting
  itself healthy the whole time. Fable verified this empirically.
- **Kubernetes API override accepts plaintext HTTP, and a missing CA file silently widens trust to
  system roots** (Codex, not independently re-verified) — both are config-dependent, not live by
  default, but worth a fail-closed check given the credentials at stake (kmaker's own Kubernetes
  bearer token plus every caller's projected token).
- **Issued SPIFFE ID encodes only the pod name, not namespace or UID** (Grok 8/10) — a pod recycle
  within the certificate's lifetime (default 24h), or two allowed service accounts in different
  namespaces happening to share a pod name, could result in two different pods holding a
  certificate for what looks like the same instance identity.
- **Kernel service-account allowlist fails open on an incomplete/empty config entry** (Grok 8/10) —
  rather than refusing to start, an incomplete `kernel_service_accounts` config silently falls back
  to allowing `schematic/kernel`.
- **`spiffe_prefix` normalization doesn't reject a value containing `/`** (Grok 9/10) — the proto
  documents the prefix as a bare SPIFFE trust-domain authority (no path separator), but nothing
  enforces that; a misconfigured prefix like `local/dev` would parse with a different trust domain
  than intended, and disagree with how the rest of the platform parses SPIFFE IDs. Also flags the
  same `local/dev` vs `local.dev` doc/code mismatch found elsewhere today.

### Low
- No client mTLS on the serving port for this `disable_auth` RPC — any network-reachable client
  can force kmaker to do a live Kubernetes TokenReview call per request (Grok) — a resource-exhaustion
  vector on the Kubernetes API, not an identity bypass.
- A 512-bit RSA key would currently be accepted as a subject key with no minimum-size check
  (Fable) — notable because every issued cert shares one SAN (`spiffe://<prefix>/kernel`), so a
  factorable weak key would compromise the collective kernel identity, not just one pod's.
- Signer failures (expired CA, entropy failure, etc.) are all reported as `InvalidArgument`
  (client error) rather than a server fault — misleading during an actual CA-expiry incident
  (Fable) — same status-code-collapsing pattern found in oauth/saml/personalauth today.
- Kubernetes API error bodies (potentially including RBAC messages naming kmaker's own service
  account) are echoed to unauthenticated callers (Fable).

## Anti-patterns / pattern drift
- Several genuinely dead code paths that would be unsafe if ever wired up without review: a
  `requested` SAN parameter that's always nil today but the function still accepts arbitrary URIs
  under the kernel prefix if it weren't; an unknown-key-usage-enum fallback that silently maps to
  `DigitalSignature`; an unused in-cluster token-review constructor (Fable).
- Config defaults are declared in three separate places (`NormalizeConfig`, generated
  `ConfigDefaults`, and a fallback inside `NewSigner`) that all set the same values independently —
  a future default change could easily update two of three (Fable).
- README documents plaintext-with-a-warning as the fallback for missing TLS material; the actual
  code refuses to start unless `KMAKER_ALLOW_PLAINTEXT=true` (Codex + Fable cross-confirmed) — an
  operator following the README would hit an unexpected outage, not the reverse (this is the safe
  direction for the mismatch to be in, but still worth fixing the docs).

## Recommended next step
Low urgency overall given the sound core path. The startup CA-sanity-check (cheap, prevents a
confusing operational failure mode) and the SPIFFE-prefix validation gap are the two worth doing
first; the rest can be batched with other low-priority cleanup whenever convenient.

## Raw per-tool output
- Codex: `codex.out.md` (4 findings)
- Fable 5.1: `fable.out.md` (6 findings, including empirical verification of the weak-key and
  CA-mismatch claims)
- Grok 4.6: `grok.out.md` (first pass, cut short by Copilot CLI's internal timeout) + `grok2.out.md`
  (completed continuation, 4 findings)
