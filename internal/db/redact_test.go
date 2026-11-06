package db

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestRedactEncodedSecrets(t *testing.T) {
	password := "a@b% c:/?+"
	userinfo := strings.TrimPrefix(url.UserPassword("", password).String(), ":")
	variants := []string{password, url.QueryEscape(password), url.PathEscape(password), userinfo}
	for _, encoded := range variants {
		for _, secret := range []string{encoded, lowerPercentEscapes(encoded)} {
			got := redact(errors.New("failed: "+secret), password).Error()
			if strings.Contains(got, secret) || !strings.Contains(got, "[redacted]") {
				t.Errorf("secret not redacted: %q", got)
			}
		}
	}
	dsn := "postgres://u:" + userinfo + "@localhost/db"
	if got := redact(errors.New(dsn), password, dsn).Error(); got != "[redacted]" {
		t.Errorf("DSN not fully redacted: %s", got)
	}
	if got := redact(errors.New("connection refused"), "").Error(); got != "connection refused" {
		t.Fatal(got)
	}
}

func lowerPercentEscapes(s string) string {
	b := []byte(s)
	for i := 0; i+2 < len(b); i++ {
		if b[i] == '%' {
			b[i+1] = strings.ToLower(string(b[i+1]))[0]
			b[i+2] = strings.ToLower(string(b[i+2]))[0]
			i += 2
		}
	}
	return string(b)
}
