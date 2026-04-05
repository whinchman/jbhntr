package web

import (
	"testing"

	"github.com/whinchman/jobhuntr/internal/config"
)

func TestBuildDriveOAuthConfig(t *testing.T) {
	t.Run("produces correct config shape", func(t *testing.T) {
		cfg := config.GoogleDriveConfig{
			ClientID:     "cid",
			ClientSecret: "csecret",
		}
		got := buildDriveOAuthConfig(cfg, "https://example.com/")
		if got.ClientID != "cid" {
			t.Errorf("ClientID = %q, want %q", got.ClientID, "cid")
		}
		if got.ClientSecret != "csecret" {
			t.Errorf("ClientSecret = %q, want %q", got.ClientSecret, "csecret")
		}
		wantRedirect := "https://example.com/auth/google-drive/callback"
		if got.RedirectURL != wantRedirect {
			t.Errorf("RedirectURL = %q, want %q", got.RedirectURL, wantRedirect)
		}
		if len(got.Scopes) != 1 || got.Scopes[0] != "https://www.googleapis.com/auth/drive.file" {
			t.Errorf("unexpected scopes: %v", got.Scopes)
		}
		if got.Endpoint.AuthURL != "https://accounts.google.com/o/oauth2/v2/auth" {
			t.Errorf("AuthURL = %q", got.Endpoint.AuthURL)
		}
		if got.Endpoint.TokenURL != "https://oauth2.googleapis.com/token" {
			t.Errorf("TokenURL = %q", got.Endpoint.TokenURL)
		}
	})

	t.Run("trims trailing slash from base URL", func(t *testing.T) {
		cfg := config.GoogleDriveConfig{ClientID: "x", ClientSecret: "y"}
		got := buildDriveOAuthConfig(cfg, "https://app.example.com/")
		want := "https://app.example.com/auth/google-drive/callback"
		if got.RedirectURL != want {
			t.Errorf("RedirectURL = %q, want %q", got.RedirectURL, want)
		}
	})
}
