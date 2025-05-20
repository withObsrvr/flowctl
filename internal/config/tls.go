package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// TLSMode represents the mode of TLS operation
type TLSMode string

const (
	// TLSModeDisabled disables TLS encryption
	TLSModeDisabled TLSMode = "disabled"
	
	// TLSModeEnabled enables TLS encryption
	TLSModeEnabled TLSMode = "enabled"
	
	// TLSModeMutual enables mutual TLS (mTLS) with client authentication
	TLSModeMutual TLSMode = "mutual"
)

// TLSConfig contains TLS configuration options
type TLSConfig struct {
	// Mode specifies the TLS mode: disabled, enabled, or mutual
	Mode TLSMode `yaml:"mode"`
	
	// CertFile is the path to the certificate file
	CertFile string `yaml:"cert_file"`
	
	// KeyFile is the path to the private key file
	KeyFile string `yaml:"key_file"`
	
	// CAFile is the path to the certificate authority file
	CAFile string `yaml:"ca_file"`
	
	// SkipVerify disables certificate verification if true
	SkipVerify bool `yaml:"skip_verify"`
	
	// ServerName is used to verify the hostname on the certificate
	ServerName string `yaml:"server_name"`
}

// DefaultTLSConfig returns a default TLS configuration with TLS disabled
func DefaultTLSConfig() *TLSConfig {
	return &TLSConfig{
		Mode:       TLSModeDisabled,
		SkipVerify: false,
	}
}

// Validate checks if the TLS configuration is valid
func (c *TLSConfig) Validate() error {
	// Skip validation if TLS is disabled
	if c.Mode == TLSModeDisabled {
		return nil
	}
	
	// For TLS and mTLS, require certificate and key
	if c.Mode == TLSModeEnabled || c.Mode == TLSModeMutual {
		if c.CertFile == "" {
			return fmt.Errorf("cert_file is required when TLS is enabled")
		}
		if c.KeyFile == "" {
			return fmt.Errorf("key_file is required when TLS is enabled")
		}
		
		// Check if certificate and key files exist
		if _, err := os.Stat(c.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("certificate file does not exist: %s", c.CertFile)
		}
		if _, err := os.Stat(c.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("key file does not exist: %s", c.KeyFile)
		}
	}
	
	// For mTLS, CA file is required (for client authentication)
	if c.Mode == TLSModeMutual && c.CAFile != "" {
		if _, err := os.Stat(c.CAFile); os.IsNotExist(err) {
			return fmt.Errorf("CA file does not exist: %s", c.CAFile)
		}
	}
	
	return nil
}

// LoadServerCredentials creates gRPC server credentials from the TLS configuration
func (c *TLSConfig) LoadServerCredentials() (grpc.ServerOption, error) {
	if c.Mode == TLSModeDisabled {
		logger.Info("TLS disabled for server")
		return nil, nil
	}
	
	logger.Info("Loading TLS credentials for server", 
		zap.String("mode", string(c.Mode)),
		zap.String("cert_file", c.CertFile),
		zap.String("key_file", c.KeyFile))
	
	// Start with standard TLS credentials
	creds, err := credentials.NewServerTLSFromFile(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
	}
	
	// For mutual TLS, set up client certificate verification
	if c.Mode == TLSModeMutual && c.CAFile != "" {
		logger.Info("Setting up mutual TLS with client certificate verification", 
			zap.String("ca_file", c.CAFile))
			
		// Create a certificate pool and add the CA certificate
		certPool := x509.NewCertPool()
		caCert, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to add CA certificate to pool")
		}
		
		// Create TLS configuration with client certificate verification
		tlsConfig := &tls.Config{
			ClientCAs:  certPool,
			ClientAuth: tls.RequireAndVerifyClientCert,
		}
		
		return grpc.Creds(credentials.NewTLS(tlsConfig)), nil
	}
	
	return grpc.Creds(creds), nil
}

// LoadClientCredentials creates gRPC client credentials from the TLS configuration
func (c *TLSConfig) LoadClientCredentials() (grpc.DialOption, error) {
	if c.Mode == TLSModeDisabled {
		logger.Info("TLS disabled for client connection")
		return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
	}
	
	logger.Info("Loading TLS credentials for client connection", 
		zap.String("mode", string(c.Mode)),
		zap.String("server_name", c.ServerName),
		zap.Bool("skip_verify", c.SkipVerify))
	
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.SkipVerify,
	}
	
	// Set server name if provided (used for certificate validation)
	if c.ServerName != "" {
		tlsConfig.ServerName = c.ServerName
	}
	
	// For client certificates (mutual TLS)
	if c.Mode == TLSModeMutual {
		logger.Info("Setting up mutual TLS client certificates", 
			zap.String("cert_file", c.CertFile),
			zap.String("key_file", c.KeyFile))
			
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate and key: %w", err)
		}
		
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	
	// Add CA certificate to trust store if provided
	if c.CAFile != "" {
		logger.Info("Adding CA certificate to trust store", zap.String("ca_file", c.CAFile))
		
		caCert, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to add CA certificate to pool")
		}
		
		tlsConfig.RootCAs = certPool
	}
	
	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}

// ResolveCertPath resolves a certificate path relative to a base directory
func ResolveCertPath(path string, baseDir string) string {
	if !filepath.IsAbs(path) && baseDir != "" {
		return filepath.Join(baseDir, path)
	}
	return path
}