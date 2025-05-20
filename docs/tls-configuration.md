# TLS Configuration for Flowctl

This document describes how to enable and configure TLS (Transport Layer Security) for secure communication between components in the Flowctl system.

## Overview

Flowctl supports three TLS modes:

1. **Disabled** - No encryption, all communication is in plaintext
2. **Enabled** - Server-side TLS for encrypted communication
3. **Mutual** - Mutual TLS (mTLS) where both server and client authenticate each other

## Server Configuration

### Using Command Line Flags

The `flowctl server` command supports the following TLS-related flags:

```
--tls-cert string      TLS certificate file
--tls-key string       TLS key file
--tls-ca-cert string   TLS CA certificate file for mutual TLS
--tls-skip-verify      Skip TLS certificate verification
```

### Examples

#### Starting a Server with TLS

```bash
flowctl server --port 8080 --tls-cert /path/to/server.crt --tls-key /path/to/server.key
```

#### Starting a Server with Mutual TLS

```bash
flowctl server --port 8080 --tls-cert /path/to/server.crt --tls-key /path/to/server.key --tls-ca-cert /path/to/ca.crt
```

## Client Configuration

Clients can be configured to use TLS when connecting to the Flowctl server through YAML configuration or programmatically.

### YAML Configuration

Add a `tls` section to your Flowctl configuration:

```yaml
version: "1.0"
log_level: "info"
source:
  type: "stellar"
  params:
    network: "testnet"
processors:
  - name: "processor1"
    plugin: "data-enrichment"
    params:
      input_format: "json"
sink:
  type: "kafka"
  params:
    bootstrap_servers: "localhost:9092"
    topic: "enriched-data"
tls:
  mode: "enabled"  # Options: disabled, enabled, mutual
  cert_file: "client.crt"
  key_file: "client.key"
  ca_file: "ca.crt"  # Required for mutual TLS
  skip_verify: false
  server_name: "flowctl-server.example.com"  # For SNI verification
```

### Programmatic Configuration

You can also configure TLS programmatically:

```go
import (
    "github.com/withobsrvr/flowctl/internal/api"
    "github.com/withobsrvr/flowctl/internal/config"
)

// Create TLS configuration
tlsConfig := &config.TLSConfig{
    Mode:       config.TLSModeEnabled,
    CertFile:   "/path/to/client.crt",
    KeyFile:    "/path/to/client.key",
    CAFile:     "/path/to/ca.crt",
    SkipVerify: false,
    ServerName: "flowctl-server.example.com",
}

// Create client options
clientOpts := &api.ClientOptions{
    ServerAddress: "localhost:8080",
    TLSConfig:     tlsConfig,
}

// Create control plane client
client, conn, err := api.CreateControlPlaneClient(clientOpts)
if err != nil {
    // Handle error
}
defer conn.Close()
```

## Certificate Generation

### Self-Signed Certificates

For testing purposes, you can generate self-signed certificates using OpenSSL:

```bash
# Generate CA key and certificate
openssl genrsa -out ca.key 4096
openssl req -new -x509 -key ca.key -sha256 -subj "/C=US/ST=CA/O=Obsrvr, Inc./CN=Obsrvr Root CA" -days 365 -out ca.crt

# Generate server key and certificate signing request (CSR)
openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr -config <(
cat <<-EOF
[req]
default_bits = 4096
prompt = no
default_md = sha256
req_extensions = req_ext
distinguished_name = dn

[dn]
C = US
ST = California
L = San Francisco
O = Obsrvr Inc.
OU = Engineering
CN = flowctl-server

[req_ext]
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = flowctl-server
IP.1 = 127.0.0.1
EOF
)

# Sign the server certificate with our CA
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out server.crt -days 365 -sha256 -extensions req_ext \
    -extfile <(
cat <<-EOF
[req_ext]
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = flowctl-server
IP.1 = 127.0.0.1
EOF
)

# Generate client key and certificate signing request
openssl genrsa -out client.key 4096
openssl req -new -key client.key -out client.csr \
    -subj "/C=US/ST=CA/L=San Francisco/O=Obsrvr Inc./OU=Engineering/CN=flowctl-client"

# Sign the client certificate with our CA
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out client.crt -days 365 -sha256
```

### Certificate Location

It's recommended to store certificates in a secure location with appropriate permissions:

- Server certificates: `/etc/flowctl/tls/` (restricted to the flowctl service user)
- Client certificates: `~/.flowctl/tls/` (restricted to the user running the client)

## Security Considerations

1. **Certificate Management**:
   - Regularly rotate certificates before they expire
   - Use a secure method to distribute certificates and keys
   - Protect private keys with appropriate file permissions

2. **Production Deployments**:
   - In production, use a proper Certificate Authority (CA) instead of self-signed certificates
   - Consider using a certificate management solution like cert-manager for Kubernetes deployments
   - Implement proper certificate revocation procedures

3. **Skip Verify**:
   - The `skip_verify` option is provided for development/testing only
   - Never use `skip_verify: true` in production environments

## Troubleshooting

### Common Issues

1. **Certificate validation failures**:
   - Verify the certificate chain is correct
   - Check certificate expiration dates
   - Ensure the server name in the certificate matches the hostname being used
   - Verify the client is using the correct CA certificate

2. **Connection refused or timeout**:
   - Check that the server is listening on the correct interface and port
   - Verify firewall rules allow TLS traffic on the specified port

### Debugging Tips

- Use OpenSSL to verify certificates:
  ```bash
  openssl verify -CAfile ca.crt server.crt
  ```

- Test TLS connection to server:
  ```bash
  openssl s_client -connect localhost:8080 -CAfile ca.crt
  ```

- Enable verbose logging:
  ```bash
  GODEBUG=tls=1 flowctl server --port 8080 --tls-cert server.crt --tls-key server.key
  ```