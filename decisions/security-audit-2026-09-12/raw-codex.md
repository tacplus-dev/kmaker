## Findings

1. **High — [tokenreview.go:68](/Users/jody/workspace/gitlab/seabeam/schematic/platform/services/kmaker/internal/kmaker/tokenreview.go:68): The Kubernetes API override accepts plaintext HTTP endpoints.**  
   **Failure scenario:** If `KMAKER_KUBE_API_URL` is configured as `http://…`, every mint request sends both kmaker’s Kubernetes bearer token and the caller’s projected token without transport protection; an on-path attacker can steal those credentials and forge TokenReview/Pod responses, causing kmaker to sign a kernel certificate for an attacker-chosen pod identity.

2. **Medium — [tokenreview.go:93](/Users/jody/workspace/gitlab/seabeam/schematic/platform/services/kmaker/internal/kmaker/tokenreview.go:93): Failure to read the configured Kubernetes CA silently widens trust to the host’s system roots.**  
   **Failure scenario:** If `KMAKER_KUBE_CA_FILE` is missing or unreadable and the API endpoint uses a publicly resolvable hostname, kmaker continues with public system CAs rather than failing closed; a publicly trusted or misissued certificate can then impersonate the TokenReview/Pod API and authorize fraudulent signing requests.

3. **Medium — [signer.go:40](/Users/jody/workspace/gitlab/seabeam/schematic/platform/services/kmaker/internal/kmaker/signer.go:40): Startup does not verify that the signing certificate is a valid CA or that its public key matches the configured private key.**  
   **Failure scenario:** A rotated or misassembled `signing_ca` containing a non-CA certificate or mismatched key starts successfully; `CreateCertificate` can emit certificates whose signatures cannot be verified through the returned chain, so kmaker reports successful issuance while every minted certificate is unusable.

4. **Low — [README.md:30](/Users/jody/workspace/gitlab/seabeam/schematic/platform/services/kmaker/README.md:30), [server.go:192](/Users/jody/workspace/gitlab/seabeam/schematic/platform/services/kmaker/internal/kmaker/server.go:192): Documentation has drifted from the fail-closed TLS behavior.**  
   **Failure scenario:** The README states that absent TLS material causes plaintext serving with a warning, while the implementation refuses startup unless `KMAKER_ALLOW_PLAINTEXT=true`; an operator following the documented bootstrap procedure gets an unexpected outage.

I found no caller-identity binding, SAN/CN injection, CA-issuance, or caller-controlled validity bypass in the mint path. Identity is derived from bound-token Pod metadata, checked against the live Pod and service account, and the signed subject, SANs, usages, CA bit, and maximum expiry are server-controlled.

No files were modified. `go test`/`go vet` could not execute because the read-only environment prevented Go from creating its temporary build directory.
