// NOTE: Go is not installed in this container. These tests require a Docker
// build (or a local Go toolchain) to execute:
//
//	docker build -t jobhuntr . && docker run --rm jobhuntr go test ./internal/web/...
//
// Run from the repo root. All tests should pass with no regressions.

package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whinchman/jobhuntr/internal/web"
)

// ─── Footer attribution (layout.html) ────────────────────────────────────────

// TestLayoutTemplate_Footer verifies that every page rendered by layout.html
// includes the footer element with the attribution text and the link to
// 217industries.com that opens in a new tab.
//
// The footer is rendered unconditionally by layout.html (outside any {{if .User}}
// guard), so an unauthenticated GET / is sufficient to exercise it.
func TestLayoutTemplate_Footer(t *testing.T) {
	ms := newMockJobStore()
	srv := web.NewServer(ms)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", resp.StatusCode)
	}
	body := string(b)

	checks := []struct {
		name    string
		snippet string
	}{
		{"footer element class", `class="app-footer"`},
		{"attribution text", "created out of spite by"},
		{"217industries.com link href", "https://www.217industries.com"},
		{"link target _blank", `target="_blank"`},
	}
	for _, c := range checks {
		if !strings.Contains(body, c.snippet) {
			t.Errorf("layout.html missing %s: snippet %q not found in body", c.name, c.snippet)
		}
	}
}

// ─── CSS smoke checks (app.css) ──────────────────────────────────────────────

// TestCSSRules_AppFooterAndCountdown fetches /static/app.css and confirms it
// contains selector blocks for .app-footer and .scrape-countdown.
func TestCSSRules_AppFooterAndCountdown(t *testing.T) {
	ms := newMockJobStore()
	srv := web.NewServer(ms)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/static/app.css")
	if err != nil {
		t.Fatalf("GET /static/app.css: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /static/app.css status = %d, want 200", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read app.css: %v", err)
	}
	css := string(b)

	selectors := []struct {
		name     string
		selector string
	}{
		{".app-footer selector", ".app-footer"},
		{".scrape-countdown selector", ".scrape-countdown"},
	}
	for _, s := range selectors {
		if !strings.Contains(css, s.selector) {
			t.Errorf("app.css missing %s: selector %q not found", s.name, s.selector)
		}
	}
}

// ─── Static CSS returns correct content-type ─────────────────────────────────

// TestStaticCSS_ContentType verifies that /static/app.css is served with a
// text/css content-type header, confirming the static file handler is wired up
// correctly.
func TestStaticCSS_ContentType(t *testing.T) {
	ms := newMockJobStore()
	srv := web.NewServer(ms)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/static/app.css")
	if err != nil {
		t.Fatalf("GET /static/app.css: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
}

// TestDashboardPage_Returns200 is a sanity check that an unauthenticated GET /
// returns 200 and a non-empty body.
func TestDashboardPage_Returns200(t *testing.T) {
	ms := newMockJobStore()
	srv := web.NewServer(ms)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, b)
	}
	if len(b) == 0 {
		t.Error("expected non-empty body for GET /")
	}
}

