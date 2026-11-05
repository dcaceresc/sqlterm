package db

import (
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/microsoft/go-mssqldb/msdsn"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sqlterm/internal/config"
	"testing"
)

func TestSQLServerCertificateModes(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()
	addr := server.Listener.Addr().String()
	p := config.Profile{Host: "127.0.0.1", User: "user", Database: "test"}
	dial := func(mode string) (*tls.Conn, error) {
		cfg, err := msdsn.Parse(sqlServerDSN(p, "password", addr, mode))
		if err != nil {
			return nil, err
		}
		return tls.Dial("tcp", addr, cfg.TLSConfig)
	}
	conn, err := dial("verify")
	if conn != nil {
		conn.Close()
	}
	if !IsCertificateError(err) {
		t.Fatalf("verify must reject the untrusted certificate: %v", err)
	}
	p.CAFile = filepath.Join(t.TempDir(), "missing.pem")
	conn, err = dial("insecure")
	if err != nil {
		t.Fatalf("explicit trust must work without a CA file: %v", err)
	}
	conn.Close()
	p.CAFile = filepath.Join(t.TempDir(), "server.pem")
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err = os.WriteFile(p.CAFile, cert, 0600); err != nil {
		t.Fatal(err)
	}
	conn, err = dial("verify")
	if err != nil {
		t.Fatalf("verify must trust the configured CA: %v", err)
	}
	conn.Close()
	cfg, err := msdsn.Parse(sqlServerDSN(p, "password", addr, "disable"))
	if err != nil || cfg.Encryption != msdsn.EncryptionDisabled {
		t.Fatalf("disable mode: %#v %v", cfg, err)
	}
}
func TestCertificateErrorClassification(t *testing.T) {
	for _, err := range []error{errors.New("TLS Handshake failed: tls: failed to verify certificate: x509: certificate signed by unknown authority"), fmt.Errorf("connect: %w", errors.New("x509: certificate is valid for another host"))} {
		if !IsCertificateError(err) {
			t.Fatal(err)
		}
	}
	for _, err := range []error{nil, errors.New("login failed"), errors.New("connection refused"), errors.New("TLS handshake timeout")} {
		if IsCertificateError(err) {
			t.Fatalf("misclassified %v", err)
		}
	}
}
