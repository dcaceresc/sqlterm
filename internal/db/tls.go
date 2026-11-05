package db

import (
	"crypto/x509"
	"errors"
	"strings"
)

// Some drivers retain x509 errors; others only include their text in a wrapper.
func IsCertificateError(err error) bool {
	if err == nil {
		return false
	}
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &authority) || errors.As(err, &hostname) || errors.As(err, &invalid) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "x509:") || strings.Contains(text, "failed to verify certificate")
}
