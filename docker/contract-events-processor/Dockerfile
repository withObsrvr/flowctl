# Dockerfile for contract-events-processor component
# Uses pre-built binary from build-in-docker.sh
#
# Build with:
#   cd docker/contract-events-processor
#   bash build-in-docker.sh
#   docker build --no-cache -t withobsrvr/contract-events-processor:v1.0.0 .

FROM scratch

# BUILD-4: OCI annotations so an image can be traced back to its source.
ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/withObsrvr/flowctl" \
      org.opencontainers.image.licenses="MPL-2.0" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.version="${VERSION}"

# Copy CA certificates for HTTPS
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the pre-built binary
COPY bin/contract-events-processor /component

# Copy metadata
COPY metadata.json /metadata.json

# Set environment defaults
ENV PORT=:50054
ENV HEALTH_PORT=8082
ENV NETWORK_PASSPHRASE="Test SDF Network ; September 2015"

# BUILD-4: never run as root. A numeric UID needs no /etc/passwd entry, which
# a scratch image does not have.
USER 65532:65532

ENTRYPOINT ["/component"]
