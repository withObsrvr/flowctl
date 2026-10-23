# Dockerfile for stellar-live-source component
# Packages a pre-built binary into a minimal OCI image for flowctl
#
# Build context should be the stellar-live-source-sdk directory:
#   docker build -t docker.io/withobsrvr/stellar-live-source:v1.0.0 \
#     -f /path/to/this/Dockerfile /path/to/stellar-live-source-sdk
#
# Prerequisites:
#   - Build the binary first: cd go && go build -o bin/stellar-live-source .
#   - Ensure metadata.json exists in the component root

FROM scratch

# BUILD-4: OCI annotations so an image can be traced back to its source.
ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/withObsrvr/flowctl" \
      org.opencontainers.image.licenses="MPL-2.0" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.version="${VERSION}"

# Copy CA certificates for HTTPS connections (needed for Stellar RPC/Horizon)
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the pre-built binary (must be at /component for flowctl resolver)
# Try multiple possible locations for the binary
COPY go/bin/stellar-live-source /component

# Copy metadata (must be at /metadata.json for flowctl resolver)
COPY metadata.json /metadata.json

# BUILD-4: never run as root. A numeric UID needs no /etc/passwd entry, which
# a scratch image does not have.
USER 65532:65532

# The binary is the entrypoint
ENTRYPOINT ["/component"]
