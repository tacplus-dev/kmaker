# Roadmap: kmaker

Owned by Jody; changed only by an agent at Jody's request. Loopers propose in PROPOSALS.md.

## Now
Bootstrap certificate authority for kernel instances: validates a pod's projected service-account token (TokenReview, Pod API) and signs its kernel certificate under the cluster signing CA.
Ported from schematic 2026-10-23. Unreleased; tacplus floor 2.4 (declared in tacplus.yaml; #1 still open).

## Next
- Nothing unblocked; see Later.

## Later
- Declare the tacplus floor 2.4 (#1), blocked by tacplus-dev/tacplus#188
- Work without SeaBeam: no gitlab.com/seabeam paths, sources or registries (#3), blocked by #1
- Patch gRPC (to >=1.83.2) and golang.org/x dependencies (#4), blocked by #1
- Add a `gates` recipe to the justfile (#5), blocked by #1
- Base image alpine:3.22 to 3.24 (#6), blocked by #5
- Release v0.1.0 and publish its image (#2), blocked by tacplus-dev/looper#12 (needs Jody's decision), #1
- Trust domain per deployment and `tac-<deployment>` namespaces; the operator renders kmaker's config (tacplus-dev/operator#3; tacplus-dev/roadmap:decisions/deployments-per-cluster-2026-10-26.md)

## Not doing
- Nothing recorded yet.
