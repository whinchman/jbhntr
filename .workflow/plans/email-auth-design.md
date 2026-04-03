# Design Specification: Email/Password Authentication UI

**Feature**: Email/password authentication — all UI screens
**Date**: 2026-04-03
**Status**: Ready for implementation

---

## 1. Design Audit Summary

The app uses **PicoCSS v2** as a base reset/component layer, overridden by `app.css` which
defines a comprehensive token system. Key patterns to reuse:

- **Login card shell**: `body.login-page` + `.login-card` + `.login-card-header` already exist and
  should be used for all auth screens (login, register, forgot password, reset password, verify email).
- **Alerts**: `.alert .alert-danger / .alert-success / .alert-info` for flash messages and inline
  field errors.
- **Buttons**: Pico primary button (indigo accent) for primary CTA; `.outline` variant for
  secondary actions.
- **Form labels + hints**: `label` + `label small` / `.field-hint` for helper text.
- **Focus ring**: `outline: 2px solid var(--color-accent)` with `box-shadow: 0 0 0 3px var(--color-accent-subtle)`.
- **Nav**: All authenticated screens use `layout.html` which includes the sticky `.app-nav`.
  Auth screens (login, register, etc.) stand alone — they use `body.login-page` and do NOT embed
  the app nav.

**No new design tokens are needed.** All screens can be built entirely from existing tokens and
component classes.

---

## 2. Shared Auth Card Shell

All five browser screens (login, register, verify-email, forgot-password, reset-password)
inherit this shell structure. It is already implemented in `login.html` for the OAuth version
and must be reused unchanged.

```
body.login-page
  article.login-card
    header.login-card-header
      h1  "JobHuntr"
      p   [subtitle — screen-specific]
    [Flash / alert block — conditional]
    [Screen-specific body]
    footer.login-card-footer   ← NEW utility class (see §8)
      [bottom links]
```

The `.login-card` is centred vertically and horizontally via `body.login-page` (flex, min-height:
100vh). Card is max-width 420px with `--shadow-lg`, white background, `--radius-lg` border.

---

## 3. Screen Specifications

### 3.1 Login Screen

**Route**: `GET /login`
**File**: `internal/web/templates/login.html` (replace current OAuth-only content)

#### Layout

```
article.login-card
  header.login-card-header
    h1        "JobHuntr"
    p         "Sign in to manage your job search"

  [.alert.alert-danger if .Flash (error) or .alert.alert-success if .FlashSuccess]

  form  method="post" action="/login"
    input[type=hidden name=csrf_token value={{.CSRFToken}}]

    label for="email"
      "Email address"
      input#email  type=email  name=email  autocomplete=email
                   placeholder="you@example.com"  required  autofocus
                   [aria-invalid="true" + aria-describedby="email-error" if field error]
    [span#email-error.field-error if field error]   ← see §7 validation

    label for="password"
      "Password"
      span.login-forgot-link   ← right-aligned, see §8
        a href="/forgot-password"  "Forgot password?"
      input#password  type=password  name=password  autocomplete=current-password
                      placeholder="••••••••"  required
                      [aria-invalid="true" if field error]

    button[type=submit]  "Sign in"   ← full width, Pico primary

  footer.login-card-footer
    p  "Don't have an account? " a href="/register" "Create one"
```

#### Copy

| Element | Text |
|---------|------|
| Title | JobHuntr |
| Subtitle | Sign in to manage your job search |
| Email label | Email address |
| Email placeholder | you@example.com |
| Password label | Password |
| Password placeholder | •••••••• |
| Forgot password link | Forgot password? |
| Submit button | Sign in |
| Footer | Don't have an account? [Create one] |

#### States

- **Default**: form fields with muted placeholders, no error indicators.
- **Field error** (e.g., wrong password): `.alert.alert-danger` above the form with a human message
  ("Invalid email or password"). Do NOT expose which field was wrong (security). Also set
  `aria-invalid="true"` on both fields.
- **Loading** (after submit): `button[aria-busy="true"]` — Pico renders a spinner automatically.
- **Success**: redirect to `/` — no success state needed on this screen.

---

### 3.2 Register Screen

**Route**: `GET /register`
**File**: `internal/web/templates/register.html` (new file)

#### Layout

```
article.login-card
  header.login-card-header
    h1   "JobHuntr"
    p    "Create your account"

  [.alert.alert-danger if .Flash]

  form  method="post" action="/register"
    input[type=hidden name=csrf_token value={{.CSRFToken}}]

    label for="display_name"
      "Display name"
      small  "How you'll appear to others"
      input#display_name  type=text  name=display_name  autocomplete=name
                          placeholder="Alex Smith"  required  autofocus
                          maxlength="100"
                          value="{{.Form.DisplayName}}"
                          [aria-invalid + aria-describedby if error]
    [span.field-error if error]

    label for="email"
      "Email address"
      input#email  type=email  name=email  autocomplete=email
                   placeholder="you@example.com"  required
                   value="{{.Form.Email}}"
                   [aria-invalid + aria-describedby if error]
    [span.field-error if error]

    label for="password"
      "Password"
      small  "At least 8 characters"
      input#password  type=password  name=password  autocomplete=new-password
                      placeholder="••••••••"  required  minlength="8"
                      [aria-invalid + aria-describedby if error]
    [span.field-error if error]

    label for="confirm_password"
      "Confirm password"
      input#confirm_password  type=password  name=confirm_password
                              autocomplete=new-password  placeholder="••••••••"
                              required
                              [aria-invalid + aria-describedby if error]
    [span.field-error if error]

    button[type=submit]  "Create account"   ← full width, Pico primary

  footer.login-card-footer
    p  "Already have an account? " a href="/login" "Sign in"
```

#### Copy

| Element | Text |
|---------|------|
| Title | JobHuntr |
| Subtitle | Create your account |
| Display name label | Display name |
| Display name hint | How you'll appear to others |
| Display name placeholder | Alex Smith |
| Email label | Email address |
| Email placeholder | you@example.com |
| Password label | Password |
| Password hint | At least 8 characters |
| Password placeholder | •••••••• |
| Confirm password label | Confirm password |
| Confirm placeholder | •••••••• |
| Submit button | Create account |
| Footer | Already have an account? [Sign in] |

#### States

- **Default**: four empty fields, no errors.
- **Field errors** (server-side validation): `.alert.alert-danger` at top with a summary, plus
  per-field `span.field-error` text and `aria-invalid` on each offending input.
  Common messages:
  - Email: "That email address is already registered."
  - Email: "Please enter a valid email address."
  - Password: "Password must be at least 8 characters."
  - Confirm: "Passwords do not match."
  - Display name: "Display name is required."
- **Sticky values**: re-populate `value=""` attributes from `.Form.*` on re-render so the user
  doesn't re-type everything. Never re-populate password fields.
- **Loading**: `button[aria-busy="true"]`.
- **Success**: redirect to `/verify-email` (see §3.3).

---

### 3.3 Verify Email Screen

**Route**: `GET /verify-email`
**File**: `internal/web/templates/verify_email.html` (new file)

This is a **holding page** — no form input. The user lands here after registration and must
check their inbox before continuing.

#### Layout

```
article.login-card
  header.login-card-header
    h1   "JobHuntr"
    p    "Check your inbox"

  [.alert.alert-success if .Flash (e.g. "Verification email resent")]
  [.alert.alert-danger  if .FlashError]

  div.verify-email-body
    p  "We've sent a verification link to"
    p  strong  "{{.Email}}"
    p.text-muted  "Click the link in that email to activate your account.
                   The link expires in 24 hours."

    hr   ← visual separator (Pico <hr> styling)

    p.text-muted  "Didn't get it? Check your spam folder, or:"

    form  method="post" action="/resend-verification"
      input[type=hidden name=csrf_token value={{.CSRFToken}}]
      button[type=submit].outline  "Resend verification email"   ← full width, outline style

  footer.login-card-footer
    p  "Wrong email? " a href="/register" "Start over"
    p  "Already verified? " a href="/login" "Sign in"
```

#### Copy

| Element | Text |
|---------|------|
| Title | JobHuntr |
| Subtitle | Check your inbox |
| Body intro | We've sent a verification link to |
| Email display | [user's email in bold] |
| Expiry note | Click the link in that email to activate your account. The link expires in 24 hours. |
| Spam note | Didn't get it? Check your spam folder, or: |
| Resend button | Resend verification email |
| Footer link 1 | Wrong email? [Start over] |
| Footer link 2 | Already verified? [Sign in] |

#### States

- **Default**: the email address shown; resend button available.
- **Resent**: `.alert.alert-success` "Verification email sent again. Please check your inbox."
- **Already verified**: redirect to `/login` with a flash "Your email is already verified."
- **Rate limited**: `.alert.alert-warning` "Please wait a moment before requesting another email."
- **Resend loading**: `button[aria-busy="true"]`.

---

### 3.4 Forgot Password Screen

**Route**: `GET /forgot-password`
**File**: `internal/web/templates/forgot_password.html` (new file)

#### Layout

```
article.login-card
  header.login-card-header
    h1   "JobHuntr"
    p    "Reset your password"

  [.alert.alert-success if .Sent]   ← shown after submission
  [.alert.alert-danger  if .Flash]

  [if not .Sent]
  form  method="post" action="/forgot-password"
    input[type=hidden name=csrf_token value={{.CSRFToken}}]

    label for="email"
      "Email address"
      small  "We'll send a reset link to this address"
      input#email  type=email  name=email  autocomplete=email
                   placeholder="you@example.com"  required  autofocus
                   value="{{.Form.Email}}"
                   [aria-invalid if error]
    [span.field-error if error]

    button[type=submit]  "Send reset link"   ← full width, Pico primary
  [/if]

  footer.login-card-footer
    p  a href="/login" "Back to sign in"
```

#### Copy

| Element | Text |
|---------|------|
| Title | JobHuntr |
| Subtitle | Reset your password |
| Email label | Email address |
| Email hint | We'll send a reset link to this address |
| Email placeholder | you@example.com |
| Submit button | Send reset link |
| Success alert | Check your inbox — we've sent a reset link to [email]. The link expires in 1 hour. |
| Footer | [Back to sign in] |

#### States

- **Default**: single email field.
- **Submitted (success)**: hide the form; show `.alert.alert-success` with the message above.
  **Do not reveal whether the email exists in the system** — always show the same success message
  regardless (security: prevents email enumeration).
- **Invalid email format**: `.alert.alert-danger` "Please enter a valid email address." + field
  `aria-invalid`.
- **Loading**: `button[aria-busy="true"]`.

---

### 3.5 Reset Password Screen

**Route**: `GET /reset-password?token=<token>`
**File**: `internal/web/templates/reset_password.html` (new file)

#### Layout

```
article.login-card
  header.login-card-header
    h1   "JobHuntr"
    p    "Choose a new password"

  [.alert.alert-danger if .Flash (invalid/expired token)]

  [if .TokenValid]
  form  method="post" action="/reset-password"
    input[type=hidden name=csrf_token value={{.CSRFToken}}]
    input[type=hidden name=token     value="{{.Token}}"]

    label for="password"
      "New password"
      small  "At least 8 characters"
      input#password  type=password  name=password  autocomplete=new-password
                      placeholder="••••••••"  required  minlength="8"  autofocus
                      [aria-invalid + aria-describedby if error]
    [span.field-error if error]

    label for="confirm_password"
      "Confirm new password"
      input#confirm_password  type=password  name=confirm_password
                              autocomplete=new-password  placeholder="••••••••"
                              required
                              [aria-invalid + aria-describedby if error]
    [span.field-error if error]

    button[type=submit]  "Set new password"   ← full width, Pico primary
  [else]
    p.text-muted  "This reset link has expired or is invalid."
    p  a href="/forgot-password" role="button"  "Request a new link"
  [/if]

  footer.login-card-footer
    [if .TokenValid]
    p  a href="/login" "Back to sign in"
    [/if]
```

#### Copy

| Element | Text |
|---------|------|
| Title | JobHuntr |
| Subtitle | Choose a new password |
| Password label | New password |
| Password hint | At least 8 characters |
| Password placeholder | •••••••• |
| Confirm label | Confirm new password |
| Confirm placeholder | •••••••• |
| Submit button | Set new password |
| Invalid token message | This reset link has expired or is invalid. |
| Invalid token CTA | [Request a new link] |
| Footer | [Back to sign in] |

#### States

- **Default (valid token)**: two password fields.
- **Invalid/expired token**: hide form; show message + link to `/forgot-password`. Token should be
  validated server-side on GET — do not render the form at all if invalid.
- **Field errors**:
  - Password too short: "Password must be at least 8 characters."
  - Passwords don't match: "Passwords do not match."
- **Success**: redirect to `/login` with `.alert.alert-success` flash "Password updated. You can now sign in."
- **Loading**: `button[aria-busy="true"]`.

---

## 4. New CSS Classes Required

The following small additions to `app.css` are needed. They are minimal — no new tokens required.

### 4.1 `.login-card-footer`

```css
/* Section 12 — LOGIN PAGE additions */

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
```

### 4.2 `.login-forgot-link`

Positions the "Forgot password?" link right-aligned within the password label row.

```css
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
```

### 4.3 `.field-error`

Per-field inline validation error message below an input.

```css
.field-error {
  display: block;
  font-size: var(--text-xs);
  color: var(--color-danger);
  margin-top: var(--space-1);
}
```

### 4.4 `.verify-email-body`

Body section of the verify-email holding page.

```css
.verify-email-body {
  text-align: center;
}

.verify-email-body p {
  margin-bottom: var(--space-2);
}

.verify-email-body hr {
  margin: var(--space-6) 0;
}
```

---

## 5. Accessibility Requirements

All auth screens must meet **WCAG 2.1 AA**.

| Requirement | Implementation |
|-------------|----------------|
| Labels on all inputs | `<label for="...">` + matching `id=` on input |
| Error association | `aria-invalid="true"` + `aria-describedby="<field>-error"` on invalid inputs |
| Role alert for flash | `role="alert"` on `.alert` divs (already in existing login.html) |
| Focus management | `autofocus` on first relevant field per screen |
| Keyboard navigation | Tab order: fields top-to-bottom, submit last, footer links after |
| Contrast | All text on white card: `--color-text` (#1a1d23 on #fff) is 14.9:1. Muted text (#6b7280 on #fff) is 4.6:1 — passes AA for normal text |
| Loading state | `aria-busy="true"` on submit button disables repeated submissions |
| Input types | `type=email`, `type=password` — enables native browser validation and mobile keyboards |
| Autocomplete | All inputs have `autocomplete` attributes (email, name, new-password, current-password) |

---

## 6. Responsive Design

All auth screens use the existing `body.login-page` centred-card layout. It is already
mobile-friendly. No additional breakpoints needed.

On mobile (< 420px viewport): the card fills the screen width with reduced horizontal padding.
Add to the login-page body:

```css
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

---

## 7. Validation States — Visual Specification

### Error state on a field

```
┌─────────────────────────────────────────┐
│ Email address                           │
│ ┌───────────────────────────────────┐   │
│ │ bad-address                 ✗     │   │  ← Pico renders red border via aria-invalid
│ └───────────────────────────────────┘   │
│ Please enter a valid email address.     │  ← .field-error (--color-danger, --text-xs)
└─────────────────────────────────────────┘
```

PicoCSS v2 automatically styles `input[aria-invalid="true"]` with a red border. No extra CSS needed
for the border treatment — only the `.field-error` text class is new.

### Alert banner (top of card)

```
┌─────────────────────────────────────────┐
│ ┊ Invalid email or password.            │  ← .alert.alert-danger
└─────────────────────────────────────────┘
```

Left border: 3px solid `--color-danger`. Background: `--color-danger-bg`. Text: `--color-danger`.

---

## 8. Email Templates

Email templates must use **plain HTML tables** — no external stylesheets, no web fonts, no
`<link>` tags. All styles must be inline. They must render correctly in Gmail, Outlook, and
Apple Mail.

### Colour values for inline use (matching app.css tokens)

| Token | Hex value |
|-------|-----------|
| `--color-bg` | `#f8f9fb` |
| `--color-surface` | `#ffffff` |
| `--color-border` | `#e2e5ec` |
| `--color-text` | `#1a1d23` |
| `--color-text-muted` | `#6b7280` |
| `--color-accent` | `#4f46e5` |
| `--color-accent-hover` | `#4338ca` |
| `--color-accent-subtle` | `#eef2ff` |

---

### 8.1 Verification Email

**Subject**: Verify your JobHuntr email address

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Verify your email — JobHuntr</title>
</head>
<body style="margin:0;padding:0;background-color:#f8f9fb;font-family:ui-sans-serif,system-ui,-apple-system,sans-serif;">

  <!-- Wrapper -->
  <table width="100%" cellpadding="0" cellspacing="0" border="0"
         style="background-color:#f8f9fb;padding:40px 16px;">
    <tr>
      <td align="center">

        <!-- Card -->
        <table width="100%" cellpadding="0" cellspacing="0" border="0"
               style="max-width:480px;background-color:#ffffff;border:1px solid #e2e5ec;
                      border-radius:12px;overflow:hidden;">

          <!-- Header -->
          <tr>
            <td style="padding:32px 40px 24px;text-align:center;
                       border-bottom:1px solid #e2e5ec;">
              <p style="margin:0;font-size:22px;font-weight:700;color:#1a1d23;">
                JobHuntr
              </p>
            </td>
          </tr>

          <!-- Body -->
          <tr>
            <td style="padding:32px 40px;">
              <p style="margin:0 0 16px;font-size:16px;color:#1a1d23;line-height:1.5;">
                Hi {{.DisplayName}},
              </p>
              <p style="margin:0 0 24px;font-size:15px;color:#6b7280;line-height:1.6;">
                Thanks for signing up. Click the button below to verify your email
                address and activate your JobHuntr account.
              </p>

              <!-- CTA Button -->
              <table width="100%" cellpadding="0" cellspacing="0" border="0">
                <tr>
                  <td align="center" style="padding-bottom:24px;">
                    <a href="{{.VerifyURL}}"
                       style="display:inline-block;padding:14px 32px;
                              background-color:#4f46e5;color:#ffffff;
                              font-size:15px;font-weight:600;text-decoration:none;
                              border-radius:8px;">
                      Verify email address
                    </a>
                  </td>
                </tr>
              </table>

              <p style="margin:0 0 8px;font-size:13px;color:#6b7280;line-height:1.5;">
                This link expires in <strong>24 hours</strong>.
              </p>
              <p style="margin:0;font-size:13px;color:#6b7280;line-height:1.5;">
                If you didn't create a JobHuntr account, you can safely ignore this email.
              </p>
            </td>
          </tr>

          <!-- Fallback URL -->
          <tr>
            <td style="padding:0 40px 24px;">
              <p style="margin:0;font-size:12px;color:#6b7280;line-height:1.5;">
                If the button doesn't work, copy and paste this link into your browser:
              </p>
              <p style="margin:4px 0 0;font-size:12px;word-break:break-all;">
                <a href="{{.VerifyURL}}"
                   style="color:#4f46e5;text-decoration:underline;">
                  {{.VerifyURL}}
                </a>
              </p>
            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="padding:16px 40px;background-color:#f8f9fb;
                       border-top:1px solid #e2e5ec;text-align:center;">
              <p style="margin:0;font-size:12px;color:#6b7280;">
                © {{.Year}} JobHuntr. All rights reserved.
              </p>
            </td>
          </tr>

        </table>
        <!-- /Card -->

      </td>
    </tr>
  </table>
  <!-- /Wrapper -->

</body>
</html>
```

**Template variables required**:

| Variable | Description |
|----------|-------------|
| `{{.DisplayName}}` | User's display name |
| `{{.VerifyURL}}` | Full absolute verification URL with token |
| `{{.Year}}` | Current year for copyright |

---

### 8.2 Password Reset Email

**Subject**: Reset your JobHuntr password

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Reset your password — JobHuntr</title>
</head>
<body style="margin:0;padding:0;background-color:#f8f9fb;font-family:ui-sans-serif,system-ui,-apple-system,sans-serif;">

  <!-- Wrapper -->
  <table width="100%" cellpadding="0" cellspacing="0" border="0"
         style="background-color:#f8f9fb;padding:40px 16px;">
    <tr>
      <td align="center">

        <!-- Card -->
        <table width="100%" cellpadding="0" cellspacing="0" border="0"
               style="max-width:480px;background-color:#ffffff;border:1px solid #e2e5ec;
                      border-radius:12px;overflow:hidden;">

          <!-- Header -->
          <tr>
            <td style="padding:32px 40px 24px;text-align:center;
                       border-bottom:1px solid #e2e5ec;">
              <p style="margin:0;font-size:22px;font-weight:700;color:#1a1d23;">
                JobHuntr
              </p>
            </td>
          </tr>

          <!-- Body -->
          <tr>
            <td style="padding:32px 40px;">
              <p style="margin:0 0 16px;font-size:16px;color:#1a1d23;line-height:1.5;">
                Hi {{.DisplayName}},
              </p>
              <p style="margin:0 0 24px;font-size:15px;color:#6b7280;line-height:1.6;">
                We received a request to reset your password. Click the button below
                to choose a new password.
              </p>

              <!-- CTA Button -->
              <table width="100%" cellpadding="0" cellspacing="0" border="0">
                <tr>
                  <td align="center" style="padding-bottom:24px;">
                    <a href="{{.ResetURL}}"
                       style="display:inline-block;padding:14px 32px;
                              background-color:#4f46e5;color:#ffffff;
                              font-size:15px;font-weight:600;text-decoration:none;
                              border-radius:8px;">
                      Reset password
                    </a>
                  </td>
                </tr>
              </table>

              <p style="margin:0 0 8px;font-size:13px;color:#6b7280;line-height:1.5;">
                This link expires in <strong>1 hour</strong>.
              </p>
              <p style="margin:0 0 24px;font-size:13px;color:#6b7280;line-height:1.5;">
                If you didn't request a password reset, you can safely ignore this email.
                Your password will remain unchanged.
              </p>

              <!-- Security notice -->
              <table width="100%" cellpadding="0" cellspacing="0" border="0">
                <tr>
                  <td style="padding:16px;background-color:#eef2ff;border-radius:8px;
                             border-left:3px solid #4f46e5;">
                    <p style="margin:0;font-size:13px;color:#1a1d23;line-height:1.5;">
                      <strong>Security tip:</strong> JobHuntr will never ask for your
                      password via email. This link can only be used once.
                    </p>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

          <!-- Fallback URL -->
          <tr>
            <td style="padding:0 40px 24px;">
              <p style="margin:0;font-size:12px;color:#6b7280;line-height:1.5;">
                If the button doesn't work, copy and paste this link into your browser:
              </p>
              <p style="margin:4px 0 0;font-size:12px;word-break:break-all;">
                <a href="{{.ResetURL}}"
                   style="color:#4f46e5;text-decoration:underline;">
                  {{.ResetURL}}
                </a>
              </p>
            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="padding:16px 40px;background-color:#f8f9fb;
                       border-top:1px solid #e2e5ec;text-align:center;">
              <p style="margin:0;font-size:12px;color:#6b7280;">
                © {{.Year}} JobHuntr. All rights reserved.
              </p>
            </td>
          </tr>

        </table>
        <!-- /Card -->

      </td>
    </tr>
  </table>
  <!-- /Wrapper -->

</body>
</html>
```

**Template variables required**:

| Variable | Description |
|----------|-------------|
| `{{.DisplayName}}` | User's display name |
| `{{.ResetURL}}` | Full absolute reset URL with one-time token |
| `{{.Year}}` | Current year for copyright |

---

## 9. Navigation Updates

The `layout.html` nav already conditionally renders "Sign in" when `.User` is nil. Two additions:

1. Add a "Register" link in the unauthenticated nav section alongside "Sign in":
   ```html
   <div class="app-nav-user">
     <a href="/login">Sign in</a>
     <a href="/register" role="button" class="btn-sm">Get started</a>
   </div>
   ```
   The `role="button"` + `btn-sm` gives the register link a subtle pill-button treatment using
   existing classes, distinguishing it from the plain "Sign in" text link.

2. After successful email verification, the "Verify your email" reminder (if any) can be shown
   as an `.alert.alert-warning` banner inside `layout.html` when `.User.EmailVerified == false`.
   This is optional but strongly recommended for UX.

---

## 10. Implementation Checklist

### New template files
- [ ] `internal/web/templates/register.html`
- [ ] `internal/web/templates/verify_email.html`
- [ ] `internal/web/templates/forgot_password.html`
- [ ] `internal/web/templates/reset_password.html`
- [ ] `internal/web/templates/email/verify_email.html` (email template)
- [ ] `internal/web/templates/email/reset_password.html` (email template)

### Modified files
- [ ] `internal/web/templates/login.html` — replace OAuth buttons with email/password form
- [ ] `internal/web/templates/layout.html` — add "Get started" nav link; optionally add verify-email banner
- [ ] `internal/web/templates/static/app.css` — add §4 classes (`.login-card-footer`, `.login-forgot-link`, `.field-error`, `.verify-email-body`, mobile card media query)

### Template data structs (backend, for coder reference)
Each screen's handler must pass a struct with at minimum:

| Screen | Required fields |
|--------|----------------|
| Login | `.Flash`, `.FlashSuccess`, `.CSRFToken` |
| Register | `.Flash`, `.CSRFToken`, `.Form.DisplayName`, `.Form.Email` |
| Verify email | `.Flash`, `.FlashError`, `.CSRFToken`, `.Email` |
| Forgot password | `.Flash`, `.CSRFToken`, `.Form.Email`, `.Sent` (bool) |
| Reset password | `.Flash`, `.CSRFToken`, `.Token`, `.TokenValid` (bool) |
| Email — verify | `.DisplayName`, `.VerifyURL`, `.Year` |
| Email — reset | `.DisplayName`, `.ResetURL`, `.Year` |
