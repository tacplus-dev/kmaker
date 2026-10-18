# Kmaker Release Image
# Layers the kmaker binary onto a minimal Alpine base.

FROM registry.gitlab.com/seabeam/schematic/containers/alpine/schematic-alpine-1-2:latest

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

COPY bin/kmaker /usr/local/bin/kmaker

ENTRYPOINT ["kmaker"]
