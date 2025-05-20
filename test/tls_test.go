package test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil" // Use os in Go 1.16+
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/withobsrvr/flowctl/internal/api"
	"github.com/withobsrvr/flowctl/internal/config"
	"github.com/withobsrvr/flowctl/internal/storage"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	pb "github.com/withobsrvr/flowctl/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/emptypb"
)

// genTLSCertificates generates self-signed certificates for testing.
// It returns the paths to the generated certificate files.
func genTLSCertificates(t *testing.T) (certFile, keyFile, caFile string) {
	// Create a temporary directory for certificates
	tmpDir, err := ioutil.TempDir("", "flowctl-tls-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Generate a CA key and certificate
	caKeyFile := filepath.Join(tmpDir, "ca.key")
	caFile = filepath.Join(tmpDir, "ca.crt")
	
	// Generate a server key and certificate
	keyFile = filepath.Join(tmpDir, "server.key")
	certFile = filepath.Join(tmpDir, "server.crt")
	
	// For testing purposes, we'll use simple bash commands to generate self-signed certs
	// In production, proper certificate generation and management is required
	// This is a simplification for testing purposes
	cmds := []string{
		// Generate CA key and certificate
		"openssl genrsa -out " + caKeyFile + " 2048",
		"openssl req -new -x509 -key " + caKeyFile + " -out " + caFile + 
			" -days 1 -subj \"/C=US/ST=CA/L=SF/O=Test/CN=Flowctl Test CA\"",
		
		// Generate server key and certificate
		"openssl genrsa -out " + keyFile + " 2048",
		"openssl req -new -key " + keyFile + " -out " + tmpDir + "/server.csr " +
			"-subj \"/C=US/ST=CA/L=SF/O=Test/CN=localhost\"",
		"openssl x509 -req -in " + tmpDir + "/server.csr -CA " + caFile + 
			" -CAkey " + caKeyFile + " -CAcreateserial -out " + certFile + 
			" -days 1 -subj \"/C=US/ST=CA/L=SF/O=Test/CN=localhost\"",
	}
	
	for _, cmd := range cmds {
		if output, err := exec.Command("bash", "-c", cmd).CombinedOutput(); err != nil {
			t.Fatalf("Failed to execute %s: %v\nOutput: %s", cmd, err, output)
		}
	}
	
	return certFile, keyFile, caFile
}

// setupTLSServer sets up a test gRPC server with TLS enabled.
func setupTLSServer(t *testing.T, tlsConfig *config.TLSConfig) (string, func()) {
	// Create a listener on a random port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	
	var serverOpts []grpc.ServerOption
	
	// Configure server with TLS if enabled
	if tlsConfig != nil && tlsConfig.Mode != config.TLSModeDisabled {
		serverCreds, err := tlsConfig.LoadServerCredentials()
		if err != nil {
			t.Fatalf("Failed to load server credentials: %v", err)
		}
		if serverCreds != nil {
			serverOpts = append(serverOpts, serverCreds)
		}
	}
	
	// Create the gRPC server
	server := grpc.NewServer(serverOpts...)
	
	// Create the control plane server
	controlPlane := api.NewControlPlaneServer(nil) // In-memory storage for tests
	pb.RegisterControlPlaneServer(server, controlPlane)
	
	// Start the server
	go func() {
		if err := server.Serve(listener); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()
	
	// Return the server address and a cleanup function
	return listener.Addr().String(), func() {
		server.Stop()
		listener.Close()
		controlPlane.Close()
	}
}

// TestTLSConnection tests connections with and without TLS.
func TestTLSConnection(t *testing.T) {
	// Skip this test in short mode
	if testing.Short() {
		t.Skip("Skipping TLS test in short mode")
	}
	
	// Initialize logger for tests
	logger.InitLogger(logger.Options{LogLevel: "error"})
	
	// Generate certificates for testing
	certFile, keyFile, caFile := genTLSCertificates(t)
	
	tests := []struct {
		name         string
		serverConfig *config.TLSConfig
		clientConfig *config.TLSConfig
		expectError  bool
	}{
		{
			name:         "No TLS",
			serverConfig: nil,
			clientConfig: nil,
			expectError:  false,
		},
		{
			name: "Server TLS, Client No TLS",
			serverConfig: &config.TLSConfig{
				Mode:     config.TLSModeEnabled,
				CertFile: certFile,
				KeyFile:  keyFile,
			},
			clientConfig: nil,
			expectError:  true, // Should fail - client using insecure, server requires TLS
		},
		{
			name: "Server TLS, Client TLS",
			serverConfig: &config.TLSConfig{
				Mode:     config.TLSModeEnabled,
				CertFile: certFile,
				KeyFile:  keyFile,
			},
			clientConfig: &config.TLSConfig{
				Mode:       config.TLSModeEnabled,
				CAFile:     caFile,
				ServerName: "localhost",
			},
			expectError: false,
		},
		{
			name: "Server mTLS, Client TLS without cert",
			serverConfig: &config.TLSConfig{
				Mode:     config.TLSModeMutual,
				CertFile: certFile,
				KeyFile:  keyFile,
				CAFile:   caFile,
			},
			clientConfig: &config.TLSConfig{
				Mode:       config.TLSModeEnabled,
				CAFile:     caFile,
				ServerName: "localhost",
			},
			expectError: true, // Should fail - client doesn't provide cert
		},
		{
			name: "TLS with Skip Verify",
			serverConfig: &config.TLSConfig{
				Mode:     config.TLSModeEnabled,
				CertFile: certFile,
				KeyFile:  keyFile,
			},
			clientConfig: &config.TLSConfig{
				Mode:       config.TLSModeEnabled,
				SkipVerify: true,
			},
			expectError: false,
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up server with appropriate TLS configuration
			serverAddr, cleanup := setupTLSServer(t, tc.serverConfig)
			defer cleanup()
			
			// Allow server to start
			time.Sleep(100 * time.Millisecond)
			
			// Create client options
			clientOpts := &api.ClientOptions{
				ServerAddress: serverAddr,
				TLSConfig:     tc.clientConfig,
			}
			
			// Create client (should succeed regardless of TLS config)
			client, conn, err := api.CreateControlPlaneClient(clientOpts)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}
			defer conn.Close()
			
			// Test the connection by making a simple RPC call
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			
			_, err = client.ListServices(ctx, &emptypb.Empty{})
			
			// Check if the result matches our expectation
			if tc.expectError && err == nil {
				t.Errorf("Expected error in test %q, but got none", tc.name)
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error in test %q, but got: %v", tc.name, err)
			}
		})
	}
}