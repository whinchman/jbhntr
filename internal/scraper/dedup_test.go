package scraper

import "testing"

func TestFingerprintJob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		title   string
		company string
		// wantEmpty signals we expect an empty string return.
		wantEmpty bool
		// sameAs contains an optional second (title, company) pair whose
		// fingerprint must equal the primary pair's fingerprint.
		sameAsTitle   string
		sameAsCompany string
		// diffFrom contains a pair whose fingerprint must differ from primary.
		diffFromTitle   string
		diffFromCompany string
	}{
		{
			name:          "case-insensitive — mixed vs lower produce same hash",
			title:         "Senior Go Engineer",
			company:       "Acme Corp",
			sameAsTitle:   "senior go engineer",
			sameAsCompany: "acme corp",
		},
		{
			name:          "extra whitespace is collapsed — leading/trailing spaces",
			title:         "Engineer",
			company:       "  Acme  Corp  ",
			sameAsTitle:   "Engineer",
			sameAsCompany: "Acme Corp",
		},
		{
			name:          "extra internal whitespace in title",
			title:         "Senior  Go   Engineer",
			company:       "Acme Corp",
			sameAsTitle:   "Senior Go Engineer",
			sameAsCompany: "Acme Corp",
		},
		{
			name:            "different companies produce different hashes",
			title:           "Engineer",
			company:         "Acme",
			diffFromTitle:   "Engineer",
			diffFromCompany: "Beta",
		},
		{
			name:            "different titles produce different hashes",
			title:           "Engineer",
			company:         "Acme",
			diffFromTitle:   "Manager",
			diffFromCompany: "Acme",
		},
		{
			name:      "empty title returns empty string",
			title:     "",
			company:   "Acme",
			wantEmpty: true,
		},
		{
			name:      "empty company returns empty string",
			title:     "Engineer",
			company:   "",
			wantEmpty: true,
		},
		{
			name:      "both empty returns empty string",
			title:     "",
			company:   "",
			wantEmpty: true,
		},
		{
			name:      "whitespace-only title returns empty string",
			title:     "   ",
			company:   "Acme",
			wantEmpty: true,
		},
		{
			name:      "whitespace-only company returns empty string",
			title:     "Engineer",
			company:   "   ",
			wantEmpty: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FingerprintJob(tc.title, tc.company)

			if tc.wantEmpty {
				if got != "" {
					t.Errorf("FingerprintJob(%q, %q) = %q, want empty", tc.title, tc.company, got)
				}
				return
			}

			// Returned value should be a non-empty 64-char hex string (SHA-256).
			if got == "" {
				t.Fatalf("FingerprintJob(%q, %q) = empty, want non-empty hash", tc.title, tc.company)
			}
			if len(got) != 64 {
				t.Errorf("FingerprintJob returned %d-char string, want 64 (SHA-256 hex)", len(got))
			}

			// Check sameAs constraint.
			if tc.sameAsTitle != "" || tc.sameAsCompany != "" {
				other := FingerprintJob(tc.sameAsTitle, tc.sameAsCompany)
				if got != other {
					t.Errorf("FingerprintJob(%q, %q) = %q\nFingerprintJob(%q, %q) = %q\nwant identical hashes",
						tc.title, tc.company, got,
						tc.sameAsTitle, tc.sameAsCompany, other)
				}
			}

			// Check diffFrom constraint.
			if tc.diffFromTitle != "" || tc.diffFromCompany != "" {
				other := FingerprintJob(tc.diffFromTitle, tc.diffFromCompany)
				if got == other {
					t.Errorf("FingerprintJob(%q, %q) and FingerprintJob(%q, %q) both returned %q, want different hashes",
						tc.title, tc.company,
						tc.diffFromTitle, tc.diffFromCompany, got)
				}
			}
		})
	}
}
