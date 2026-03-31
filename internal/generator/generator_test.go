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

func TestGenerate(t *testing.T) {
	ctx := context.Background()

	t.Run("parses resume and cover letter from response", func(t *testing.T) {
		resumeHTML := "<html><body><h1>Resume</h1></body></html>"
		coverHTML := "<html><body><h1>Cover Letter</h1></body></html>"
		responseText := resumeHTML + "\n" + separator + "\n" + coverHTML

		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(makeAnthropicResponse(responseText))
		})

		gotResume, gotCover, err := gen.Generate(ctx, sampleJob(), sampleResume)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if !strings.Contains(gotResume, "Resume") {
			t.Errorf("resumeHTML = %q, want to contain 'Resume'", gotResume)
		}
		if !strings.Contains(gotCover, "Cover Letter") {
			t.Errorf("coverHTML = %q, want to contain 'Cover Letter'", gotCover)
		}
	})

	t.Run("returns error when separator is missing", func(t *testing.T) {
		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(makeAnthropicResponse("<html>only resume, no separator</html>"))
		})

		_, _, err := gen.Generate(ctx, sampleJob(), sampleResume)
		if err == nil {
			t.Error("Generate() expected error for missing separator, got nil")
		}
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		gen := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"internal server error"}}`))
		})

		_, _, err := gen.Generate(ctx, sampleJob(), sampleResume)
		if err == nil {
			t.Error("Generate() expected error for API failure, got nil")
		}
	})
}
