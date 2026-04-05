package web

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/oauth2"
)

// deriveKey converts an arbitrary-length secret string into a 32-byte AES-256
// key by computing its SHA-256 hash.
func deriveKey(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// encryptToken serializes tok to JSON then encrypts it with AES-GCM.
// key must be exactly 32 bytes (AES-256).
// Returns a base64url-encoded string of: nonce || ciphertext.
func encryptToken(key []byte, tok *oauth2.Token) (string, error) {
	plaintext, err := json.Marshal(tok)
	if err != nil {
		return "", fmt.Errorf("encryptToken: marshal: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("encryptToken: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encryptToken: new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encryptToken: generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// decryptToken decrypts a base64url-encoded nonce||ciphertext produced by
// encryptToken and deserializes the result into an oauth2.Token.
func decryptToken(key []byte, encoded string) (*oauth2.Token, error) {
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decryptToken: base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("decryptToken: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("decryptToken: new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("decryptToken: ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryptToken: decrypt: %w", err)
	}

	var tok oauth2.Token
	if err := json.Unmarshal(plaintext, &tok); err != nil {
		return nil, fmt.Errorf("decryptToken: unmarshal: %w", err)
	}
	return &tok, nil
}
