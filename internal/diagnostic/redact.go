// Package diagnostic contains helpers for presenting technical errors safely.
package diagnostic

import (
	"errors"
	"net/url"
	"sort"
	"strings"
)

const redacted = "[redacted]"

// Redact removes raw and URL-encoded forms of secrets from an error. Matches
// are replaced in one pass, longest first, so a complete DSN is removed before
// a password contained within it and replacement text is never scanned again.
func Redact(err error, secrets ...string) error {
	if err == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var variants []string
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		userinfo := strings.TrimPrefix(url.UserPassword("", secret).String(), ":")
		for _, variant := range []string{
			secret,
			url.QueryEscape(secret),
			url.PathEscape(secret),
			userinfo,
		} {
			for _, candidate := range []string{variant, lowerPercentEscapes(variant)} {
				if candidate == "" {
					continue
				}
				if _, ok := seen[candidate]; ok {
					continue
				}
				seen[candidate] = struct{}{}
				variants = append(variants, candidate)
			}
		}
	}
	if len(variants) == 0 {
		return errors.New(err.Error())
	}

	sort.SliceStable(variants, func(i, j int) bool {
		return len(variants[i]) > len(variants[j])
	})
	pairs := make([]string, 0, len(variants)*2)
	for _, variant := range variants {
		pairs = append(pairs, variant, redacted)
	}
	return errors.New(strings.NewReplacer(pairs...).Replace(err.Error()))
}

func lowerPercentEscapes(value string) string {
	b := []byte(value)
	for i := 0; i+2 < len(b); i++ {
		if b[i] != '%' {
			continue
		}
		b[i+1] = lowerHex(b[i+1])
		b[i+2] = lowerHex(b[i+2])
		i += 2
	}
	return string(b)
}

func lowerHex(b byte) byte {
	if b >= 'A' && b <= 'F' {
		return b + ('a' - 'A')
	}
	return b
}
