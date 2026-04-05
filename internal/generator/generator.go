// Package generator produces tailored resumes and cover letters via Claude API.
package generator

import (
	"context"
	"fmt"
	"strings"

	anthropic "github.com/liushuangls/go-anthropic/v2"
	"github.com/whinchman/jobhuntr/internal/models"
)

// Generator generates resume and cover letter content for a job listing.
type Generator interface {
	Generate(ctx context.Context, job models.Job, baseResume string) (resumeMD, resumeHTML, coverMD, coverHTML string, err error)
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
// It returns the resume and cover letter in both Markdown and HTML formats.
func (g *AnthropicGenerator) Generate(ctx context.Context, job models.Job, baseResume string) (string, string, string, string, error) {
	userMsg := fmt.Sprintf(userPromptTemplate,
		job.Title, job.Company, job.Location, job.Salary, job.Description, baseResume,
	)

	resp, err := g.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model:     anthropic.Model(g.model),
		MaxTokens: 16384,
		System:    systemPrompt,
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.MessageContent{
				anthropic.NewTextMessageContent(userMsg),
			}},
		},
	})
	if err != nil {
		return "", "", "", "", fmt.Errorf("generator: claude api: %w", err)
	}

	if len(resp.Content) == 0 {
		return "", "", "", "", fmt.Errorf("generator: empty response from claude")
	}

	raw := resp.Content[0].GetText()

	// Extract each section using the separator constants.
	resumeMD, err := extractSection(raw, sepResumeMD, sepResumeHTML)
	if err != nil {
		return "", "", "", "", fmt.Errorf("generator: %w", err)
	}
	resumeHTML, err := extractSection(raw, sepResumeHTML, sepCoverMD)
	if err != nil {
		return "", "", "", "", fmt.Errorf("generator: %w", err)
	}
	coverMD, err := extractSection(raw, sepCoverMD, sepCoverHTML)
	if err != nil {
		return "", "", "", "", fmt.Errorf("generator: %w", err)
	}

	// Cover HTML is everything after the last separator.
	idx := strings.Index(raw, sepCoverHTML)
	if idx < 0 {
		return "", "", "", "", fmt.Errorf("generator: separator %q not found in response", sepCoverHTML)
	}
	coverHTML := strings.TrimSpace(raw[idx+len(sepCoverHTML):])

	return resumeMD, resumeHTML, coverMD, coverHTML, nil
}

// extractSection returns the trimmed text between startSep and endSep in s.
func extractSection(s, startSep, endSep string) (string, error) {
	start := strings.Index(s, startSep)
	if start < 0 {
		return "", fmt.Errorf("separator %q not found in response", startSep)
	}
	rest := s[start+len(startSep):]
	end := strings.Index(rest, endSep)
	if end < 0 {
		return "", fmt.Errorf("separator %q not found in response", endSep)
	}
	return strings.TrimSpace(rest[:end]), nil
}
