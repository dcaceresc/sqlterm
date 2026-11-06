package diagnostic

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestRedactEncodedSecrets(t *testing.T) {
	password := "a@b% c:/?+"
	userinfo := strings.TrimPrefix(url.UserPassword("", password).String(), ":")
	variants := []string{
		password,
		url.QueryEscape(password),
		url.PathEscape(password),
		userinfo,
	}
	for _, encoded := range variants {
		for _, secret := range []string{encoded, lowerPercentEscapes(encoded)} {
			got := Redact(errors.New("failed: "+secret), password).Error()
			if strings.Contains(got, secret) || !strings.Contains(got, redacted) {
				t.Errorf("secret not redacted: %q", got)
			}
		}
	}
}

func TestRedactUsesLongestSinglePassMatches(t *testing.T) {
	password := "red"
	dsn := "postgres://user:red@localhost/database"
	if got := Redact(errors.New(dsn), password, dsn).Error(); got != redacted {
		t.Fatalf("complete DSN was not redacted first: %q", got)
	}
	if got := Redact(errors.New("connection refused"), "").Error(); got != "connection refused" {
		t.Fatalf("non-secret error changed: %q", got)
	}
	if got := Redact(nil, password); got != nil {
		t.Fatalf("nil error changed: %v", got)
	}
}
