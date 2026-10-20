# Dockerfile for duckdb-consumer component
# Packages a pre-built CGO binary. Debian slim rather than scratch, because
# DuckDB needs glibc.
#
# Build context should be the duckdb-consumer directory, containing:
#   - bin/duckdb-consumer (the compiled binary)
#   - metadata.json (component metadata)
#
# Build the binary first, on a glibc-based system (not NixOS):
#   CGO_ENABLED=1 go build -o bin/duckdb-consumer .
#
# Then:
#   docker build -t docker.io/withobsrvr/duckdb-consumer:v1.0.0 \
#     --build-arg VERSION=v1.0.0 --build-arg REVISION=$(git rev-parse HEAD) \
#     -f docker/duckdb-consumer/Dockerfile /path/to/duckdb-consumer

FROM debian:bookworm-slim

# BUILD-4: OCI annotations so an image can be traced back to its source.
ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/withObsrvr/flowctl" \
      org.opencontainers.image.licenses="MPL-2.0" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.version="${VERSION}"

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# BUILD-4: never run as root. The data directory is owned by this user, since
# the component writes its DuckDB file there.
RUN useradd --system --uid 65532 --user-group --no-create-home --shell /usr/sbin/nologin flowctl \
    && mkdir -p /data \
    && chown flowctl:flowctl /data

WORKDIR /app

# Copy the pre-built binary
COPY bin/duckdb-consumer /component

# Copy metadata
COPY metadata.json /metadata.json

# Set environment defaults
ENV DUCKDB_PATH=/data/stellar.duckdb
ENV PORT=:50053
ENV HEALTH_PORT=8090

USER flowctl:flowctl

# The binary is the entrypoint
ENTRYPOINT ["/component"]
