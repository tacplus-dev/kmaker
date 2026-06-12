package grpcserver

import (
	"strings"

	wrapperSpiffe "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/libs/spiffe"
)

// IsAtOrBelow reports whether target is the same as caller or a strict
// path-segment descendant. Inputs are namespace paths (e.g. "/tenants/acme") or
// full SPIFFE URIs (e.g. "spiffe://example.com/tenants/acme"); two URIs must
// share a trust domain, and a URI never matches a bare path.
//
// This is the NAMESPACE (authorization-scope) relation, delegated to the
// canonical generated libs/spiffe helper; it is distinct from the identity-path
// relation (spiffe.IsSpiffeAtOrBelow) and the two must not be cross-applied.
// The match is path-segment-aware: "/tenants/foo" is NOT at-or-below
// "/tenants/foob". Malformed URIs fail closed (return false).
func IsAtOrBelow(caller, target string) bool {
	if caller == "" || target == "" {
		return false
	}
	prefix := wrapperSpiffe.Scheme + "://"
	callerIsURI := strings.HasPrefix(caller, prefix)
	targetIsURI := strings.HasPrefix(target, prefix)
	if callerIsURI != targetIsURI {
		// An identity URI and a bare namespace path are different universes.
		return false
	}
	if callerIsURI {
		c, err := wrapperSpiffe.ParseSpiffeID(caller)
		if err != nil {
			return false
		}
		t, err := wrapperSpiffe.ParseSpiffeID(target)
		if err != nil {
			return false
		}
		if c.TrustDomain != t.TrustDomain {
			return false
		}
		return wrapperSpiffe.IsAtOrBelow(c.Namespace, t.Namespace)
	}
	return wrapperSpiffe.IsAtOrBelow(caller, target)
}
