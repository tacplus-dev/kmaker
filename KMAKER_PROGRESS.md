# Kmaker Progress

## 2026-06-16

- Aligned handwritten kmaker code with the governed proto contract:
  - `SignKernelCert` is now `MintKernelCert`.
  - signer and handler tests use the generated `MintKernelCertRequest`.
- Regenerated the Go model from `/Users/jody/workspace/cavalier/proto` using the
  source-built tacplus binary after fixing the generator.
- Verified generated service manager code now calls `MintX509Cert` instead of
  the legacy `X509Mint`.
- Verified generated SPIFFE fixture tests skip cleanly when the optional shared
  fixture is not present in the downstream repo.
- Local verification passed:
  - tacplus generation completed.
  - `go test ./...`.
- Push is pending the tacplus `v1.2.206` tag pipeline so downstream CI can
  rebuild from a tagged generator image.
