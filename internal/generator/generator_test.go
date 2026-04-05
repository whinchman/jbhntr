package generator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	anthropic "github.com/liushuangls/go-anthropic/v2"
	"github.com/whinchman/jobhuntr/internal/models"
)

// capturedRequest holds the decoded Anthropic API request body for inspection.
type capturedRequest struct {
	MaxTokens int `json:"max_tokens"`
}

func makeAnthropicResponse(content string) map[string]any {
	return map[string]any{
		"id":   "msg_test",
		"type": "message",
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": content},
		},
		"model":        "claude-sonnet-4-20250514",
		"stop_reason":  "end_turn",
		"usage":        map[string]any{"input_tokens": 100, "output_tokens": 200},
	}
}

func newTestGenerator(t *testing.T, handler http.HandlerFunc) *AnthropicGenerator {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := anthropic.NewClient("test-key", anthropic.WithBaseURL(srv.URL+"/"))
	return &AnthropicGenerator{
		client: client,
		model:  "claude-sonnet-4-20250514",
	}
}

func sampleJob() models.Job {
	return models.Job{
		Title:       "Senior Go Engineer",
		Company:     "Acme Corp",
		Location:    "Remote",
		Salary:      "$150k",
		Description: "Build scalable systems in Go.",
	}
}

const sampleResume = `# Jane Doe
## Experience
- Software Engineer at FooCo
`

// buildFourSectionResponse assembles a valid four-section response using the
// separator constants defined in prompts.go.
func buildFourSectionResponse(resumeMD, resumeHTML, coverMD, coverHTML string) string {
	return sepResumeMD + "\n" + resumeMD + "\n" +
		sepResumeHTML + "\n" + resumeHTML + "\n" +
		sepCoverMD + "\n" + coverMD + "\n" +
		sepCoverHTML + "\n" + coverHTML
}

func TestGenerate(t *testing.T) {
	ctx := context.Background()

	t.Run("parses all four sections from response", func(t *testing.T) {
		resumeMD := "# Resume in Markdown"
		resumeHTML := "<html><body><h1>Resume</h1></body></html>"
		coverMD := "# Cover Letter in Markdown"
		coverHTML := "<html><body><h1>Cover Letter</h1></body></html>"
		responseText := buildFourSectionResponse(resumeMD, resumeHTML, coverMD, coverHTML)

		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(makeAnthropicResponse(responseText))
		})

		gotResumeMD, gotResumeHTML, gotCoverMD, gotCoverHTML, err := gen.Generate(ctx, sampleJob(), sampleResume)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if !strings.Contains(gotResumeMD, "Resume in Markdown") {
			t.Errorf("resumeMD = %q, want to contain 'Resume in Markdown'", gotResumeMD)
		}
		if !strings.Contains(gotResumeHTML, "Resume") {
			t.Errorf("resumeHTML = %q, want to contain 'Resume'", gotResumeHTML)
		}
		if !strings.Contains(gotCoverMD, "Cover Letter in Markdown") {
			t.Errorf("coverMD = %q, want to contain 'Cover Letter in Markdown'", gotCoverMD)
		}
		if !strings.Contains(gotCoverHTML, "Cover Letter") {
			t.Errorf("coverHTML = %q, want to contain 'Cover Letter'", gotCoverHTML)
		}
	})

	t.Run("returns error when RESUME_MD separator is missing", func(t *testing.T) {
		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(makeAnthropicResponse("<html>only resume, no separators</html>"))
		})

		_, _, _, _, err := gen.Generate(ctx, sampleJob(), sampleResume)
		if err == nil {
			t.Error("Generate() expected error for missing separator, got nil")
		}
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"internal server error"}}`))
		})

		_, _, _, _, err := gen.Generate(ctx, sampleJob(), sampleResume)
		if err == nil {
			t.Error("Generate() expected error for API failure, got nil")
		}
	})

	t.Run("sends MaxTokens=16384 in API request", func(t *testing.T) {
		// This test verifies the fix for bug-006: MaxTokens must be 16384 so that
		// large HTML sections do not truncate the response before the ---COVER_MD---
		// separator is reached.
		var gotMaxTokens int
		responseText := buildFourSectionResponse("# Resume", "<h1>R</h1>", "# Cover", "<h1>C</h1>")

		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			var req capturedRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				gotMaxTokens = req.MaxTokens
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(makeAnthropicResponse(responseText))
		})

		if _, _, _, _, err := gen.Generate(ctx, sampleJob(), sampleResume); err != nil {
			t.Fatalf("Generate() unexpected error: %v", err)
		}
		if gotMaxTokens != 16384 {
			t.Errorf("MaxTokens sent to API = %d, want 16384 (bug-006 fix)", gotMaxTokens)
		}
	})

	t.Run("returns error when RESUME_HTML separator is missing", func(t *testing.T) {
		// Response has RESUME_MD but not RESUME_HTML — extractSection should fail.
		badResponse := sepResumeMD + "\n# Resume only, no further separators"
		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(makeAnthropicResponse(badResponse))
		})

		_, _, _, _, err := gen.Generate(ctx, sampleJob(), sampleResume)
		if err == nil {
			t.Error("Generate() expected error for missing RESUME_HTML separator, got nil")
		}
		if !strings.Contains(err.Error(), "not found in response") {
			t.Errorf("error message = %q, want to contain 'not found in response'", err.Error())
		}
	})

	t.Run("returns error when COVER_MD separator is missing", func(t *testing.T) {
		// Response has RESUME_MD and RESUME_HTML but no COVER_MD.
		badResponse := sepResumeMD + "\n# Resume\n" + sepResumeHTML + "\n<h1>R</h1>"
		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(makeAnthropicResponse(badResponse))
		})

		_, _, _, _, err := gen.Generate(ctx, sampleJob(), sampleResume)
		if err == nil {
			t.Error("Generate() expected error for missing COVER_MD separator, got nil")
		}
	})

	t.Run("returns error when COVER_HTML separator is missing", func(t *testing.T) {
		// Response has RESUME_MD, RESUME_HTML, COVER_MD but no COVER_HTML.
		badResponse := sepResumeMD + "\n# Resume\n" + sepResumeHTML + "\n<h1>R</h1>\n" + sepCoverMD + "\n# Cover"
		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(makeAnthropicResponse(badResponse))
		})

		_, _, _, _, err := gen.Generate(ctx, sampleJob(), sampleResume)
		if err == nil {
			t.Error("Generate() expected error for missing COVER_HTML separator, got nil")
		}
	})

	t.Run("returns error on empty response content", func(t *testing.T) {
		// API returns a valid 200 but with an empty content array.
		emptyResp := map[string]any{
			"id":          "msg_empty",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]any{},
			"model":       "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 0},
		}
		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(emptyResp)
		})

		_, _, _, _, err := gen.Generate(ctx, sampleJob(), sampleResume)
		if err == nil {
			t.Error("Generate() expected error for empty content, got nil")
		}
		if !strings.Contains(err.Error(), "empty response") {
			t.Errorf("error message = %q, want to contain 'empty response'", err.Error())
		}
	})

	t.Run("trims surrounding whitespace from all four sections", func(t *testing.T) {
		// Sections with extra leading/trailing whitespace should be trimmed.
		responseText := buildFourSectionResponse(
			"  \n# Resume\n  ",
			"  <h1>Resume</h1>  \n",
			"\n# Cover\n",
			"\n<h1>Cover</h1>\n  ",
		)
		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(makeAnthropicResponse(responseText))
		})

		rMD, rHTML, cMD, cHTML, err := gen.Generate(ctx, sampleJob(), sampleResume)
		if err != nil {
			t.Fatalf("Generate() unexpected error: %v", err)
		}
		if strings.HasPrefix(rMD, " ") || strings.HasSuffix(rMD, " ") || strings.HasPrefix(rMD, "\n") {
			t.Errorf("resumeMD not trimmed: %q", rMD)
		}
		if strings.HasPrefix(rHTML, " ") || strings.HasSuffix(rHTML, " ") || strings.HasSuffix(rHTML, "\n") {
			t.Errorf("resumeHTML not trimmed: %q", rHTML)
		}
		if strings.HasPrefix(cMD, "\n") || strings.HasSuffix(cMD, "\n") {
			t.Errorf("coverMD not trimmed: %q", cMD)
		}
		if strings.HasPrefix(cHTML, "\n") || strings.HasSuffix(cHTML, " ") {
			t.Errorf("coverHTML not trimmed: %q", cHTML)
		}
	})
}

// TestExtractSection tests the internal extractSection helper directly.
func TestExtractSection(t *testing.T) {
	t.Run("extracts content between two separators", func(t *testing.T) {
		s := "---START---\nhello world\n---END---\nignored"
		got, err := extractSection(s, "---START---", "---END---")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "hello world" {
			t.Errorf("got %q, want %q", got, "hello world")
		}
	})

	t.Run("returns error when start separator is missing", func(t *testing.T) {
		_, err := extractSection("no separators here", "---START---", "---END---")
		if err == nil {
			t.Error("expected error for missing start separator, got nil")
		}
		if !strings.Contains(err.Error(), "---START---") {
			t.Errorf("error %q should mention missing separator", err.Error())
		}
	})

	t.Run("returns error when end separator is missing", func(t *testing.T) {
		_, err := extractSection("---START---\nsome content", "---START---", "---END---")
		if err == nil {
			t.Error("expected error for missing end separator, got nil")
		}
		if !strings.Contains(err.Error(), "---END---") {
			t.Errorf("error %q should mention missing end separator", err.Error())
		}
	})

	t.Run("trims whitespace from extracted content", func(t *testing.T) {
		s := "---A---\n  trimmed  \n---B---"
		got, err := extractSection(s, "---A---", "---B---")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "trimmed" {
			t.Errorf("got %q, want %q", got, "trimmed")
		}
	})

	t.Run("handles empty content between separators", func(t *testing.T) {
		s := "---A---\n---B---"
		got, err := extractSection(s, "---A---", "---B---")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})
}
