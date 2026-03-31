package generator

import (
	"context"
	"fmt"
	"strings"

	anthropic "github.com/liushuangls/go-anthropic/v2"
	"github.com/whinchman/jobhuntr/internal/models"
)

// Summarizer produces a short summary and extracts salary from a job description.
type Summarizer interface {
	Summarize(ctx context.Context, job models.Job) (summary, salary string, err error)
}

const summarizeSystemPrompt = `You summarize job listings. Given a job title, company, and description, respond with exactly two lines:
LINE 1: A 1-2 sentence summary of the role and key requirements.
LINE 2: The salary or compensation range if mentioned anywhere in the description, or "N/A" if not found.

Do not include labels like "Summary:" or "Salary:" — just the two lines of content, separated by a single newline.`

const summarizeUserTemplate = `Title: %s
Company: %s

Description:
%s`

// AnthropicSummarizer implements Summarizer using Claude.
type AnthropicSummarizer struct {
	client *anthropic.Client
	model  string
}

// NewAnthropicSummarizer creates a summarizer. Uses a fast/cheap model by default.
func NewAnthropicSummarizer(apiKey, model string) *AnthropicSummarizer {
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	return &AnthropicSummarizer{
		client: anthropic.NewClient(apiKey),
		model:  model,
	}
}

// Summarize calls Claude to produce a short summary and extract salary info.
func (s *AnthropicSummarizer) Summarize(ctx context.Context, job models.Job) (string, string, error) {
	userMsg := fmt.Sprintf(summarizeUserTemplate, job.Title, job.Company, job.Description)

	resp, err := s.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model:     anthropic.Model(s.model),
		MaxTokens: 256,
		System:    summarizeSystemPrompt,
		Messages: []anthropic.Message{
			{Role: anthropic.RoleUser, Content: []anthropic.MessageContent{
				anthropic.NewTextMessageContent(userMsg),
			}},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("summarizer: claude api: %w", err)
	}

	if len(resp.Content) == 0 {
		return "", "", fmt.Errorf("summarizer: empty response")
	}

	raw := strings.TrimSpace(resp.Content[0].GetText())
	lines := strings.SplitN(raw, "\n", 2)

	summary := strings.TrimSpace(lines[0])
	salary := ""
	if len(lines) > 1 {
		salary = strings.TrimSpace(lines[1])
		if strings.EqualFold(salary, "N/A") || strings.EqualFold(salary, "n/a") {
			salary = ""
		}
	}

	return summary, salary, nil
}
