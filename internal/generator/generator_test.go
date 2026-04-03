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
}
