✗ Read (a70f8ba1)
  └ Failed

● Todo started correctness

✗ Search (grep)
  │ "uri_sans|IsCa|dns_names|instance_id" in **/*.{go}
  │ (~/workspace/gitlab/seabeam/schematic/platform/services/kmaker/internal)
  └ Permission denied and could not request permission from user

✔ Todo completed 2 items
  │ sec-review
  └ correctness

Mint path is recently hardened (caller SANs/CA bits stripped; identity from TokenReview + live Pod). Short list of remaining high-confidence issues:

| # | Severity | File | Lines | Vulnerability | Confidence |
|---|----------|------|-------|---------------|------------|
| 1 | 🟡 MEDIUM | `internal/kmaker/handler.go` | 141–146, 191–198 | Issued SPIFFE ID is pod **name** only (no namespace/UID) | 8/10 |
| 2 | 🟡 MEDIUM | `internal/kmaker/config.go` | 86–101 | Kernel SA allowlist fail-opens to `schematic/kernel` | 8/10 |
| 3 | 🟡 MEDIUM | `internal/kmaker/signer.go` | 266–274, 176–178 | `spiffe_prefix` `/` not rejected; URI host is only first segment | 9/10 |
| 4 | ⚪ LOW | `internal/kmaker/handler.go` | 49–51; `server.go` 196–208 | Unauthenticated RPC (no client mTLS) can hammer TokenReview | 8/10 |

**1.** Cert SAN is `spiffe://<td>/kernel/<podName>`. Token UID is checked at mint, then dropped. Pod recycle within `kernel_cert_lifetime_seconds` (default 24h), or two allowed SAs in different namespaces with the same pod name, share one instance identity. Old cert still valid.

**2.** Empty/incomplete `kernel_service_accounts` entries are skipped; empty map silently allows `schematic/kernel` instead of refusing to start.

**3.** Proto says prefix is SPIFFE authority (no `/`). `normalizeSpiffePrefix` keeps slashes. `spiffe://local/dev/kernel` parses as trust domain `local`, path `/dev/kernel`. README default `local/dev` vs code `local.dev`. Platform `ParseSpiffeID` will disagree.

**4.** `MintKernelCert` is `disable_auth`; serving TLS has no `ClientAuth`. Any client that can reach the port forces kube TokenReview.

**Not bugs (hardened):** caller DNS/URI/CA/keyUsage; identity from TokenReview extras + live Pod; exp capped; CA key not returned.

**Drift:** README still says missing `tls` → plaintext; code requires `KMAKER_ALLOW_PLAINTEXT=true`. Dead `mapKeyUsage` / `kernelURISANs(requested)` would be unsafe if wired.

Follow-up:
1. Fix highest severity issues
2. Fix all issues
3. Commit a summary of findings

