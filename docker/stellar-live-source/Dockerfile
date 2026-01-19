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

# Copy CA certificates for HTTPS connections (needed for Stellar RPC/Horizon)
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the pre-built binary (must be at /component for flowctl resolver)
# Try multiple possible locations for the binary
COPY go/bin/stellar-live-source /component

# Copy metadata (must be at /metadata.json for flowctl resolver)
COPY metadata.json /metadata.json

# The binary is the entrypoint
ENTRYPOINT ["/component"]
