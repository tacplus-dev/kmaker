All evidence is in. Writing up the findings now.

**Bottom line: the identity-binding path is sound.** I found no way for a caller to obtain a certificate for an identity that is not theirs. The handler derives pod name and UID from the API server's TokenReview extras, cross-checks them against a live Pod GET in the service account's namespace, rejects any caller-supplied pod or instance metadata that disagrees, and the signer rebuilds the template from scratch so caller SANs, CA bits, key usages and issuer never reach the certificate. Validity is capped by the configured lifetime and the CA's own NotAfter. The signer is immutable after construction, so there are no signing races. Tests pass and `go vet` is clean.

Findings, ranked:

1. **Medium, correctness. The signing CA is never sanity-checked at startup** (`internal/kmaker/signer.go:40-47`). NewSigner parses the cert and key separately and never confirms they match, nor that chain[0] is a CA. I verified on Go 1.26 that a mismatched key is only caught inside CreateCertificate, and a non-CA parent is not caught at all. Scenario: a config reseed pairs the wrong key with the CA cert, or puts a leaf in chain[0]. kmaker starts, flips health to SERVING, and then either fails every mint or issues leaves that fail chain verification at every peer. A one-time check in NewSigner would turn this into a startup failure.

2. **Low, security. Weak subject keys are accepted** (`internal/kmaker/signer.go:68,128`). ParsePKIXPublicKey plus CreateCertificate accept a 512-bit RSA key, verified empirically. Every issued cert also carries the shared SAN `spiffe://<prefix>/kernel`, so a factorable key holds the collective kernel identity, not just one pod's. Exposure needs a pod that chooses a weak key, so this is low, but a minimum key size check is cheap.

3. **Low, correctness. All signer failures surface as InvalidArgument** (`internal/kmaker/handler.go:86`). "Signing CA is expired", entropy failure and CreateCertificate errors are server faults but are reported as client errors. Kernel pods treating InvalidArgument as non-retryable will give up with a misleading message during a CA-expiry incident.

4. **Low, information disclosure. API server error bodies are echoed to unauthenticated callers** (`internal/kmaker/handler.go:58` and `:117`, sourced from `tokenreview.go:171,216`). Up to 4KB of the Kubernetes response, including RBAC messages naming kmaker's service account, goes back over the wire before any authentication succeeds.

5. **Anti-pattern. Dead code that would fail open if revived** (`internal/kmaker/signer.go:150-161`, `:344`, `:387-445`). The `requested` parameter of kernelURISANs is always nil but still accepts any URI under the kernel prefix. mapKeyUsage maps every unknown enum value to DigitalSignature. mapExtKeyUsages, validateDNSNames, the HTTPTokenReviewer alias, NewInClusterTokenReviewer and the retained `token` field (`tokenreview.go:61,65,124`) have no callers.

6. **Pattern drift. Defaults live in three places and disagree with the README.** NormalizeConfig (`config.go:56-81`), the generated ConfigDefaults (`server.go:56`), and a fallback inside NewSigner (`signer.go:50-52`) all set the same values. The README documents the spiffe prefix default as `local/dev` while code uses `local.dev`, and the handler tests use `local/dev`. The fallback in AllowedServiceAccounts (`config.go:105-107`) is unreachable in Run because NormalizeConfig and ValidateConfig already guarantee at least one complete entry.

Nothing was modified. The only file I wrote is a throwaway Go probe in the session scratchpad.
