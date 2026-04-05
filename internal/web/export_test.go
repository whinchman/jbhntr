// export_test.go exposes unexported web package symbols for use by external
// test packages (package web_test). Only compiled during testing.
package web

import "golang.org/x/oauth2"

// DeriveKeyForTest exposes deriveKey for package web_test.
func DeriveKeyForTest(secret string) []byte {
	return deriveKey(secret)
}

// EncryptTokenForTest exposes encryptToken for package web_test.
func EncryptTokenForTest(key []byte, tok *oauth2.Token) (string, error) {
	return encryptToken(key, tok)
}

// DecryptTokenForTest exposes decryptToken for package web_test.
func DecryptTokenForTest(key []byte, encoded string) (*oauth2.Token, error) {
	return decryptToken(key, encoded)
}
