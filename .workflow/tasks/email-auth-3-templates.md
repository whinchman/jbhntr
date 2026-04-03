# Task: email-auth-3-templates

- **Type**: coder
- **Status**: pending
- **Repo**: .
- **Parallel Group**: 3
- **Branch**: feature/email-auth-3-templates
- **Source Item**: email-auth (email/password authentication)
- **Dependencies**: email-auth-2-server-wiring

## Description

Implement all HTML templates and CSS changes for email/password authentication. This covers 4 new browser templates, 2 email templates, updates to `login.html` and `layout.html`, and 4 new CSS rule sets in `app.css`. Also add a 5th screen (`verify_email.html`) as a holding page for post-registration.

All templates use PicoCSS v2 base + existing `app.css` tokens. No new design tokens.

## Acceptance Criteria

- [ ] `internal/web/templates/login.html` updated: email+password form, CSRF hidden input, pre-fill email, OAuth section only when `len .Providers > 0`, `forgot password` link, `.login-card-footer`
- [ ] `internal/web/templates/register.html` created: 4 fields (display_name, email, password, confirm_password), sticky form values, CSRF, accessibility attributes, `.login-card-footer`
- [ ] `internal/web/templates/verify_email.html` created: holding page with email display, resend form, `.verify-email-body`
- [ ] `internal/web/templates/forgot_password.html` created: email field, conditional form hiding when `.Sent`, CSRF
- [ ] `internal/web/templates/reset_password.html` created: conditional rendering based on `.TokenValid`, 2 password fields, hidden token input, CSRF
- [ ] `internal/web/templates/email/verify_email.html` created: inline-styled HTML email
- [ ] `internal/web/templates/email/reset_password.html` created: inline-styled HTML email
- [ ] `internal/web/templates/layout.html` updated: "Get started" nav link for unauthenticated users alongside "Sign in"
- [ ] `internal/web/templates/static/app.css` updated: 4 new rule sets in Section 12, mobile media query
- [ ] All templates render without error when exercised through the server

## Interface Contracts

### Template data structs (defined in `internal/web/auth.go` by `email-auth-3-handlers` task or `email-auth-2-server-wiring` task)

Each template must match the exact field names from these structs:

**`login.html`** receives `loginData`:
```
.Providers    []string   — loop to render OAuth buttons; if empty, omit the OAuth section entirely
.Flash        string     — error message (alert-danger)
.FlashSuccess string     — success message (alert-success)
.CSRFToken    string     — value for hidden CSRF input (name="gorilla.csrf.Token")
.Email        string     — pre-fill email input on error
```

**`register.html`** receives `registerData`:
```
.Flash              string   — error summary (alert-danger)
.CSRFToken          string
.Form.DisplayName   string   — sticky value
.Form.Email         string   — sticky value
```

**`verify_email.html`** receives `verifyEmailData`:
```
.Flash      string   — success (alert-success)
.FlashError string   — error (alert-danger)
.CSRFToken  string
.Email      string   — display the user's email
```

**`forgot_password.html`** receives `forgotPasswordData`:
```
.Flash        string   — success message shown after submit (alert-success)
.CSRFToken    string
.Sent         bool     — when true, hide the form and show success alert
.Form.Email   string   — sticky value
```

**`reset_password.html`** receives `resetPasswordData`:
```
.Flash      string   — error (alert-danger) or success (alert-success)
.CSRFToken  string
.Token      string   — hidden field value
.TokenValid bool     — when false, show "link expired" message instead of form
```

**`email/verify_email.html`** receives `verifyEmailTemplateData`:
```
.DisplayName  string
.VerifyURL    string   — full absolute URL
.Year         int
```

**`email/reset_password.html`** receives `resetEmailTemplateData`:
```
.DisplayName  string
.ResetURL     string   — full absolute URL
.Year         int
```

### CSRF field name

All forms must include exactly:
```html
<input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">
```

## Context

### Existing template to model: `login.html`

Read `internal/web/templates/login.html` as the reference for the auth card shell pattern (`body.login-page`, `article.login-card`, `header.login-card-header`). All new templates must use the same shell.

### Updated `login.html`

Replace the current OAuth-only content. New structure:

```html
{{template "base" .}}
{{define "content"}}
<body class="login-page">
  <article class="login-card">
    <header class="login-card-header">
      <h1>JobHuntr</h1>
      <p>Sign in to manage your job search</p>
    </header>

    {{if .Flash}}
    <div class="alert alert-danger" role="alert">{{.Flash}}</div>
    {{end}}
    {{if .FlashSuccess}}
    <div class="alert alert-success" role="alert">{{.FlashSuccess}}</div>
    {{end}}

    <form method="post" action="/login">
      <input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">

      <label for="email">
        Email address
        <input id="email" type="email" name="email" autocomplete="email"
               placeholder="you@example.com" required autofocus
               value="{{.Email}}"
               {{if .Flash}}aria-invalid="true"{{end}}>
      </label>

      <label for="password">
        Password
        <span class="login-forgot-link">
          <a href="/forgot-password">Forgot password?</a>
        </span>
        <input id="password" type="password" name="password" autocomplete="current-password"
               placeholder="••••••••" required
               {{if .Flash}}aria-invalid="true"{{end}}>
      </label>

      <button type="submit">Sign in</button>
    </form>

    {{if .Providers}}
    <div class="login-divider"><span>or</span></div>
    {{range .Providers}}
    <a href="/auth/{{.}}" role="button" class="outline">Continue with {{.}}</a>
    {{end}}
    {{end}}

    <footer class="login-card-footer">
      <p>Don't have an account? <a href="/register">Create one</a></p>
    </footer>
  </article>
</body>
{{end}}
```

### `register.html`

```html
{{template "base" .}}
{{define "content"}}
<body class="login-page">
  <article class="login-card">
    <header class="login-card-header">
      <h1>JobHuntr</h1>
      <p>Create your account</p>
    </header>

    {{if .Flash}}
    <div class="alert alert-danger" role="alert">{{.Flash}}</div>
    {{end}}

    <form method="post" action="/register">
      <input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">

      <label for="display_name">
        Display name
        <small>How you'll appear to others</small>
        <input id="display_name" type="text" name="display_name" autocomplete="name"
               placeholder="Alex Smith" required autofocus maxlength="100"
               value="{{.Form.DisplayName}}">
      </label>

      <label for="email">
        Email address
        <input id="email" type="email" name="email" autocomplete="email"
               placeholder="you@example.com" required
               value="{{.Form.Email}}">
      </label>

      <label for="password">
        Password
        <small>At least 8 characters</small>
        <input id="password" type="password" name="password" autocomplete="new-password"
               placeholder="••••••••" required minlength="8">
      </label>

      <label for="confirm_password">
        Confirm password
        <input id="confirm_password" type="password" name="confirm_password"
               autocomplete="new-password" placeholder="••••••••" required>
      </label>

      <button type="submit">Create account</button>
    </form>

    <footer class="login-card-footer">
      <p>Already have an account? <a href="/login">Sign in</a></p>
    </footer>
  </article>
</body>
{{end}}
```

### `verify_email.html`

```html
{{template "base" .}}
{{define "content"}}
<body class="login-page">
  <article class="login-card">
    <header class="login-card-header">
      <h1>JobHuntr</h1>
      <p>Check your inbox</p>
    </header>

    {{if .Flash}}
    <div class="alert alert-success" role="alert">{{.Flash}}</div>
    {{end}}
    {{if .FlashError}}
    <div class="alert alert-danger" role="alert">{{.FlashError}}</div>
    {{end}}

    <div class="verify-email-body">
      <p>We've sent a verification link to</p>
      <p><strong>{{.Email}}</strong></p>
      <p class="text-muted">Click the link in that email to activate your account. The link expires in 24 hours.</p>

      <hr>

      <p class="text-muted">Didn't get it? Check your spam folder, or:</p>

      <form method="post" action="/resend-verification">
        <input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">
        <button type="submit" class="outline">Resend verification email</button>
      </form>
    </div>

    <footer class="login-card-footer">
      <p>Wrong email? <a href="/register">Start over</a></p>
      <p>Already verified? <a href="/login">Sign in</a></p>
    </footer>
  </article>
</body>
{{end}}
```

### `forgot_password.html`

```html
{{template "base" .}}
{{define "content"}}
<body class="login-page">
  <article class="login-card">
    <header class="login-card-header">
      <h1>JobHuntr</h1>
      <p>Reset your password</p>
    </header>

    {{if .Flash}}
    <div class="alert alert-success" role="alert">{{.Flash}}</div>
    {{end}}

    {{if not .Sent}}
    <form method="post" action="/forgot-password">
      <input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">

      <label for="email">
        Email address
        <small>We'll send a reset link to this address</small>
        <input id="email" type="email" name="email" autocomplete="email"
               placeholder="you@example.com" required autofocus
               value="{{.Form.Email}}">
      </label>

      <button type="submit">Send reset link</button>
    </form>
    {{end}}

    <footer class="login-card-footer">
      <p><a href="/login">Back to sign in</a></p>
    </footer>
  </article>
</body>
{{end}}
```

Note: Go templates use `{{if not .Sent}}` syntax — ensure it compiles (may need `{{if not .Sent}}` or `{{if eq .Sent false}}`; use whichever the existing templates use).

### `reset_password.html`

```html
{{template "base" .}}
{{define "content"}}
<body class="login-page">
  <article class="login-card">
    <header class="login-card-header">
      <h1>JobHuntr</h1>
      <p>Choose a new password</p>
    </header>

    {{if .Flash}}
    <div class="alert alert-danger" role="alert">{{.Flash}}</div>
    {{end}}

    {{if .TokenValid}}
    <form method="post" action="/reset-password">
      <input type="hidden" name="gorilla.csrf.Token" value="{{.CSRFToken}}">
      <input type="hidden" name="token" value="{{.Token}}">

      <label for="password">
        New password
        <small>At least 8 characters</small>
        <input id="password" type="password" name="password" autocomplete="new-password"
               placeholder="••••••••" required minlength="8" autofocus>
      </label>

      <label for="confirm_password">
        Confirm new password
        <input id="confirm_password" type="password" name="confirm_password"
               autocomplete="new-password" placeholder="••••••••" required>
      </label>

      <button type="submit">Set new password</button>
    </form>
    {{else}}
    <p class="text-muted">This reset link has expired or is invalid.</p>
    <p><a href="/forgot-password" role="button">Request a new link</a></p>
    {{end}}

    {{if .TokenValid}}
    <footer class="login-card-footer">
      <p><a href="/login">Back to sign in</a></p>
    </footer>
    {{end}}
  </article>
</body>
{{end}}
```

### Email templates

Create directory `internal/web/templates/email/` if it doesn't exist.

**`email/verify_email.html`** — copy the full HTML from the design spec (§8.1). Template variables: `{{.DisplayName}}`, `{{.VerifyURL}}`, `{{.Year}}`.

**`email/reset_password.html`** — copy the full HTML from the design spec (§8.2). Template variables: `{{.DisplayName}}`, `{{.ResetURL}}`, `{{.Year}}`.

The full HTML for both email templates is in `/workspace/.workflow/plans/email-auth-design.md` sections 8.1 and 8.2. Copy them verbatim.

### CSS additions (`internal/web/templates/static/app.css`)

Add at the end under a comment `/* Section 12 — LOGIN PAGE additions */`:

```css
/* Section 12 — LOGIN PAGE additions */

/* 12.1 Login card footer */
.login-card-footer {
  margin-top: var(--space-6);
  padding-top: var(--space-4);
  border-top: 1px solid var(--color-border);
  text-align: center;
}
.login-card-footer p {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  margin: 0 0 var(--space-2);
}
.login-card-footer p:last-child {
  margin-bottom: 0;
}
.login-card-footer a {
  color: var(--color-accent);
  font-weight: var(--weight-medium);
  text-decoration: none;
}
.login-card-footer a:hover {
  color: var(--color-accent-hover);
  text-decoration: underline;
}

/* 12.2 Forgot password link (right-aligned in password label) */
.login-forgot-link {
  float: right;
  font-size: var(--text-xs);
  font-weight: var(--weight-normal);
}
.login-forgot-link a {
  color: var(--color-text-muted);
  text-decoration: none;
}
.login-forgot-link a:hover {
  color: var(--color-accent);
  text-decoration: underline;
}

/* 12.3 Per-field inline validation error */
.field-error {
  display: block;
  font-size: var(--text-xs);
  color: var(--color-danger);
  margin-top: var(--space-1);
}

/* 12.4 Verify-email holding page body */
.verify-email-body {
  text-align: center;
}
.verify-email-body p {
  margin-bottom: var(--space-2);
}
.verify-email-body hr {
  margin: var(--space-6) 0;
}

/* 12.5 Mobile: full-bleed card on small screens */
@media (max-width: 480px) {
  .login-card {
    margin: 0;
    border-radius: 0;
    border-left: none;
    border-right: none;
    min-height: 100vh;
    box-shadow: none;
  }
}
```

### `layout.html` update

Find the unauthenticated nav section (where `Sign in` link is rendered) and add a `Get started` link:

```html
<div class="app-nav-user">
  <a href="/login">Sign in</a>
  <a href="/register" role="button" class="btn-sm">Get started</a>
</div>
```

Read the existing `layout.html` to find the exact conditional block and insert alongside the existing "Sign in" link.

## Notes

<!-- implementing agent fills this in -->
