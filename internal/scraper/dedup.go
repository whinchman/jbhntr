package scraper

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var whitespaceRe = regexp.MustCompile(`\s+`)

// FingerprintJob returns a hex-encoded SHA-256 fingerprint derived from
// normalized title and company name. Returns "" if either is empty.
//
// Normalization:
//  1. strings.ToLower + strings.TrimSpace
//  2. Collapse internal whitespace runs to a single space
//  3. Concatenate as title + "|" + company before hashing
func FingerprintJob(title, company string) string {
	t := normalize(title)
	c := normalize(company)
	if t == "" || c == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(t + "|" + c))
	return hex.EncodeToString(sum[:])
}

func normalize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = whitespaceRe.ReplaceAllString(s, " ")
	return s
}
