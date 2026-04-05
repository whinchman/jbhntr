package web

import (
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestEncryptDecryptToken_RoundTrip(t *testing.T) {
	secret := "test-session-secret-for-unit-tests"
	key := deriveKey(secret)

	expiry := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	original := &oauth2.Token{
		AccessToken:  "access-abc-123",
		TokenType:    "Bearer",
		RefreshToken: "refresh-xyz-789",
		Expiry:       expiry,
	}

	encoded, err := encryptToken(key, original)
	if err != nil {
		t.Fatalf("encryptToken: unexpected error: %v", err)
	}
	if encoded == "" {
		t.Fatal("encryptToken: returned empty string")
	}

	decoded, err := decryptToken(key, encoded)
	if err != nil {
		t.Fatalf("decryptToken: unexpected error: %v", err)
	}

	if decoded.AccessToken != original.AccessToken {
		t.Errorf("AccessToken mismatch: got %q, want %q", decoded.AccessToken, original.AccessToken)
	}
	if decoded.TokenType != original.TokenType {
		t.Errorf("TokenType mismatch: got %q, want %q", decoded.TokenType, original.TokenType)
	}
	if decoded.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken mismatch: got %q, want %q", decoded.RefreshToken, original.RefreshToken)
	}
	if !decoded.Expiry.Equal(original.Expiry) {
		t.Errorf("Expiry mismatch: got %v, want %v", decoded.Expiry, original.Expiry)
	}
}

func TestEncryptToken_ProducesDifferentOutputEachCall(t *testing.T) {
	key := deriveKey("some-secret")
	tok := &oauth2.Token{AccessToken: "same-token"}

	enc1, err := encryptToken(key, tok)
	if err != nil {
		t.Fatalf("first encryptToken: %v", err)
	}
	enc2, err := encryptToken(key, tok)
	if err != nil {
		t.Fatalf("second encryptToken: %v", err)
	}

	// Each call generates a fresh random nonce, so results must differ.
	if enc1 == enc2 {
		t.Error("expected different ciphertexts for two separate encryptions (nonce must be random)")
	}
}

func TestDecryptToken_TamperedCiphertext(t *testing.T) {
	key := deriveKey("test-secret")
	tok := &oauth2.Token{AccessToken: "tamper-me"}

	encoded, err := encryptToken(key, tok)
	if err != nil {
		t.Fatalf("encryptToken: %v", err)
	}

	// Flip the last byte of the base64 string to produce an invalid ciphertext.
	tampered := []byte(encoded)
	last := len(tampered) - 1
	if tampered[last] == 'A' {
		tampered[last] = 'B'
	} else {
		tampered[last] = 'A'
	}

	_, err = decryptToken(key, string(tampered))
	if err == nil {
		t.Error("expected error when decrypting tampered ciphertext, got nil")
	}
}

func TestDecryptToken_WrongKey(t *testing.T) {
	key1 := deriveKey("correct-secret")
	key2 := deriveKey("wrong-secret")

	tok := &oauth2.Token{AccessToken: "secret-data"}
	encoded, err := encryptToken(key1, tok)
	if err != nil {
		t.Fatalf("encryptToken: %v", err)
	}

	_, err = decryptToken(key2, encoded)
	if err == nil {
		t.Error("expected error when decrypting with wrong key, got nil")
	}
}

func TestDecryptToken_InvalidBase64(t *testing.T) {
	key := deriveKey("any-secret")
	_, err := decryptToken(key, "not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64 input, got nil")
	}
}

func TestDecryptToken_TooShort(t *testing.T) {
	key := deriveKey("any-secret")
	// Encode a very short byte slice (less than nonce size = 12 bytes).
	shortEncoded := "dG9vc2hvcnQ=" // "tooshort" in base64 (8 bytes)
	_, err := decryptToken(key, shortEncoded)
	if err == nil {
		t.Error("expected error for too-short ciphertext, got nil")
	}
}

func TestDeriveKey_Is32Bytes(t *testing.T) {
	for _, secret := range []string{"", "short", "a very long secret string that exceeds 32 characters in length"} {
		key := deriveKey(secret)
		if len(key) != 32 {
			t.Errorf("deriveKey(%q): got %d bytes, want 32", secret, len(key))
		}
	}
}

// TestEncryptDecryptToken_ZeroExpiry verifies that a token with a zero-value
// Expiry (time.Time{}) survives a round-trip correctly. Zero Expiry is valid
// for tokens that do not expire (e.g. long-lived service-account tokens).
func TestEncryptDecryptToken_ZeroExpiry(t *testing.T) {
	key := deriveKey("zero-expiry-secret")
	original := &oauth2.Token{
		AccessToken:  "access-zero-expiry",
		RefreshToken: "refresh-zero-expiry",
		// Expiry intentionally left as zero value (time.Time{}).
	}

	encoded, err := encryptToken(key, original)
	if err != nil {
		t.Fatalf("encryptToken: %v", err)
	}

	decoded, err := decryptToken(key, encoded)
	if err != nil {
		t.Fatalf("decryptToken: %v", err)
	}

	if decoded.AccessToken != original.AccessToken {
		t.Errorf("AccessToken mismatch: got %q, want %q", decoded.AccessToken, original.AccessToken)
	}
	if decoded.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken mismatch: got %q, want %q", decoded.RefreshToken, original.RefreshToken)
	}
	if !decoded.Expiry.IsZero() {
		t.Errorf("Expiry should be zero, got %v", decoded.Expiry)
	}
}

// TestEncryptDecryptToken_OnlyRefreshToken verifies that a token containing
// only a RefreshToken (and empty AccessToken) round-trips correctly. This
// represents the initial offline-access grant before the first refresh.
func TestEncryptDecryptToken_OnlyRefreshToken(t *testing.T) {
	key := deriveKey("refresh-only-secret")
	original := &oauth2.Token{
		RefreshToken: "offline-refresh-token",
	}

	encoded, err := encryptToken(key, original)
	if err != nil {
		t.Fatalf("encryptToken: %v", err)
	}

	decoded, err := decryptToken(key, encoded)
	if err != nil {
		t.Fatalf("decryptToken: %v", err)
	}

	if decoded.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken mismatch: got %q, want %q", decoded.RefreshToken, original.RefreshToken)
	}
	if decoded.AccessToken != "" {
		t.Errorf("AccessToken should be empty, got %q", decoded.AccessToken)
	}
}
