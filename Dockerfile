# Kmaker Release Image
# Layers the kmaker binary onto a minimal Alpine base.

# The base is a build argument: a fleet may hand in its own hardened base, the
# default is the public one (no private registry in a public contract's repo).
ARG BASE_IMAGE=alpine:3.22
FROM ${BASE_IMAGE}

ARG BUILD_DATE
ARG VCS_REF
ARG VERSION

LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.authors="Seabeam LLC <cloudctlplane@seabeam.com>" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.vendor="Seabeam LLC" \
      org.opencontainers.image.title="Kmaker" \
      org.opencontainers.image.description="Kernel bootstrap certificate signing service for Schematic control plane"

RUN apk add --no-cache ca-certificates
COPY bin/kmaker /usr/local/bin/kmaker

ENTRYPOINT ["kmaker"]
