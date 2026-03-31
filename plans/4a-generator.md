# Plan: 4A — Claude API Generator

## Overview
Implement resume + cover letter generation using the Anthropic Claude API.

## Steps

### Step 1: Install go-anthropic/v2
- go get github.com/liushuangls/go-anthropic/v2

### Step 2: internal/generator/prompts.go
Constants/template for system prompt and user message format.

### Step 3: internal/generator/generator.go
Generator interface:
  Generate(ctx, job models.Job, baseResume string) (resumeHTML, coverHTML string, error)

Struct: AnthropicGenerator with claude client, model string.
- Build messages: system prompt (output two HTML docs separated by "---SEPARATOR---"),
  user message with job details + base resume
- Call claude-sonnet-4-20250514, parse response by splitting on "---SEPARATOR---"
- Return trimmed HTML strings

### Step 4: Tests (internal/generator/generator_test.go)
Use a mock HTTP server (httptest) to stub the Anthropic API.
- Successful generation: verify separator parsing, returns two HTML strings
- Missing separator: returns error
- API error: returns wrapped error

## Files Created/Modified
- internal/generator/prompts.go (new)
- internal/generator/generator.go (replaces doc.go)
- internal/generator/generator_test.go (new)
