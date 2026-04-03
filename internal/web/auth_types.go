package web

// registerData is the template data for the register page.
type registerData struct {
	Flash     string
	CSRFToken string
	Form      struct {
		DisplayName string
		Email       string
	}
}

// forgotPasswordData is the template data for the forgot-password page.
type forgotPasswordData struct {
	Flash     string
	CSRFToken string
	Sent      bool
	Form      struct {
		Email string
	}
}

// resetPasswordData is the template data for the reset-password page.
type resetPasswordData struct {
	Flash      string
	CSRFToken  string
	Token      string
	TokenValid bool
}

// verifyEmailData is the template data for the verify-email page.
type verifyEmailData struct {
	Flash      string
	FlashError string
	CSRFToken  string
	Email      string
}
