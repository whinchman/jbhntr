package models

import "testing"

func TestJobStatusValid(t *testing.T) {
	valid := []JobStatus{
		StatusDiscovered,
		StatusNotified,
		StatusApproved,
		StatusRejected,
		StatusGenerating,
		StatusComplete,
		StatusFailed,
	}

	for _, s := range valid {
		t.Run(string(s), func(t *testing.T) {
			if !s.Valid() {
				t.Errorf("JobStatus(%q).Valid() = false, want true", s)
			}
		})
	}

	t.Run("invalid status", func(t *testing.T) {
		if JobStatus("unknown").Valid() {
			t.Error("JobStatus(\"unknown\").Valid() = true, want false")
		}
	})

	t.Run("empty string is invalid", func(t *testing.T) {
		if JobStatus("").Valid() {
			t.Error("JobStatus(\"\").Valid() = true, want false")
		}
	})
}
