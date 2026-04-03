# Task: email-auth-1-config

- **Type**: coder
- **Status**: pending
- **Repo**: .
- **Parallel Group**: 1
- **Branch**: feature/email-auth-1-config
- **Source Item**: email-auth (email/password authentication)
- **Dependencies**: none

## Description

Extend `internal/config/config.go` with two additions:
1. A new `SMTPConfig` struct and a top-level `SMTP` field on `Config`
2. A nested `OAuthConfig` struct (with `Enabled bool`) inside `AuthConfig`

Also update `internal/config/config_test.go` (or add tests there) to cover the new fields. This is a small, self-contained change with no dependencies on other Group 1 tasks.

## Acceptance Criteria

- [ ] `SMTPConfig` struct exists with `Host`, `Port`, `Username`, `Password`, `From` fields
- [ ] `Config.SMTP SMTPConfig` field added with yaml tag `smtp`
- [ ] `OAuthConfig` struct with `Enabled bool` exists
- [ ] `AuthConfig.OAuth OAuthConfig` field added with yaml tag `oauth`
- [ ] Existing `AuthConfig.Providers` field and `AuthConfig.SessionSecret` field are **unchanged**
- [ ] `config_test.go` covers the new fields (parse from a YAML string, assert values)
- [ ] `go test ./internal/config/...` passes

## Interface Contracts

The `SMTPConfig` values flow to `main.go` where they are used to construct `mailer.NewSMTPMailer(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.From)`.

The `OAuthConfig.Enabled` flag is consumed in `internal/web/server.go` (Group 2 task) as:
```go
if cfg.Auth.OAuth.Enabled {
    srv.oauthProviders = oauthProviders(cfg.Auth, cfg.Server.BaseURL)
}
```

## Context

### `internal/config/config.go` changes

Current `AuthConfig` (do not remove any existing fields):
```go
type AuthConfig struct {
    SessionSecret string          `yaml:"session_secret"`
    Providers     ProvidersConfig `yaml:"providers"`
    // ... any other existing fields
}
```

Add `OAuthConfig` nested struct and `OAuth` field:
```go
type OAuthConfig struct {
    Enabled bool `yaml:"enabled"`
}

type AuthConfig struct {
    SessionSecret string          `yaml:"session_secret"`
    OAuth         OAuthConfig     `yaml:"oauth"`
    Providers     ProvidersConfig `yaml:"providers"`   // unchanged
}
```

Add `SMTPConfig` struct and top-level field:
```go
type SMTPConfig struct {
    Host     string `yaml:"host"`
    Port     int    `yaml:"port"`
    Username string `yaml:"username"`
    Password string `yaml:"password"`
    From     string `yaml:"from"`
}
```

Add to `Config` struct:
```go
SMTP SMTPConfig `yaml:"smtp"`
```

### Example config additions (for reference / comment in code)

```yaml
auth:
  session_secret: ${SESSION_SECRET}
  oauth:
    enabled: false   # set true to show OAuth buttons on login page
  providers:
    google:
      client_id: ""
      client_secret: ""
    github:
      client_id: ""
      client_secret: ""

smtp:
  host: ${SMTP_HOST}
  port: 587
  username: ${SMTP_USERNAME}
  password: ${SMTP_PASSWORD}
  from: noreply@example.com
```

### Tests

Read `internal/config/config_test.go` for existing test style. Add test cases that:
- Parse a YAML string containing `smtp.host`, `smtp.port`, `smtp.username`, `smtp.password`, `smtp.from` and assert values are populated correctly on the `Config` struct
- Parse a YAML string with `auth.oauth.enabled: true` and assert `cfg.Auth.OAuth.Enabled == true`
- Parse a YAML string without `smtp` block and assert `cfg.SMTP` is zero value (no panic)

## Notes

<!-- implementing agent fills this in -->
