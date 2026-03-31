// Package generator produces tailored resumes and cover letters via Claude API.
package generator

import (
	"context"
	"fmt"
	"strings"

	anthropic "github.com/liushuangls/go-anthropic/v2"
	"github.com/whinchman/jobhuntr/internal/models"
)

// Generator generates resume and cover letter HTML for a job listing.
type Generator interface {
	Generate(ctx context.Context, job models.Job, baseResume string) (resumeHTML, coverHTML string, err error)
}

// AnthropicGenerator implements Generator using the Anthropic Claude API.
type AnthropicGenerator struct {
	client *anthropic.Client
	model  string
}

// NewAnthropicGenerator creates an AnthropicGenerator.
func NewAnthropicGenerator(apiKey, model string) *AnthropicGenerator {
	return &AnthropicGenerator{
		client: anthropic.NewClient(apiKey),
		model:  model,
	}
}

// Generate calls Claude to produce a tailored resume and cover letter for job.
func (g *AnthropicGenerator) Generate(ctx context.Context, job models.Job, baseResume string) (string, string, error) {
	userMsg := fmt.Sprintf(userPromptTemplate,
		job.Title, job.Company, job.Location, job.Salary, job.Description, baseResume,
	)

	resp, err := g.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model:     anthropic.Model(g.model),
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.MessageContent{
				anthropic.NewTextMessageContent(userMsg),
			}},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("generator: claude api: %w", err)
	}

	if len(resp.Content) == 0 {
		return "", "", fmt.Errorf("generator: empty response from claude")
	}

	raw := resp.Content[0].GetText()
	parts := strings.SplitN(raw, separator, 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("generator: separator %q not found in response", separator)
	}

	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
