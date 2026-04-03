// Package exporter converts Markdown content to office document formats.
package exporter

import (
	"bytes"
	"strings"

	"github.com/gomutex/godocx"
	docxpkg "github.com/gomutex/godocx/docx"
)

// ToDocx converts a Markdown string to a DOCX file and returns the raw bytes.
// Supported Markdown subset: ATX headings (# H1, ## H2, ### H3), bold (**text**),
// italic (_text_ or *text*), unordered lists (- item or * item), and plain paragraphs.
// Unknown elements are rendered as plain text.
func ToDocx(md string) ([]byte, error) {
	doc, err := godocx.NewDocument()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(md, "\n")

	// We accumulate consecutive non-blank non-heading non-list lines into a
	// paragraph buffer and flush it when we hit a blank line or a structural
	// element.
	var paraLines []string

	flushParagraph := func() {
		if len(paraLines) == 0 {
			return
		}
		text := strings.Join(paraLines, " ")
		paraLines = nil
		p := doc.AddParagraph("")
		addInlineRuns(p, text)
	}

	for _, raw := range lines {
		line := raw

		// Blank line: flush accumulated paragraph.
		if strings.TrimSpace(line) == "" {
			flushParagraph()
			continue
		}

		// ATX headings: # H1, ## H2, ### H3 (check longer prefixes first)
		if strings.HasPrefix(line, "### ") {
			flushParagraph()
			_, _ = doc.AddHeading(strings.TrimPrefix(line, "### "), 3)
			continue
		}
		if strings.HasPrefix(line, "## ") {
			flushParagraph()
			_, _ = doc.AddHeading(strings.TrimPrefix(line, "## "), 2)
			continue
		}
		if strings.HasPrefix(line, "# ") {
			flushParagraph()
			_, _ = doc.AddHeading(strings.TrimPrefix(line, "# "), 1)
			continue
		}

		// Unordered list items: "- " or "* "
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			flushParagraph()
			text := line[2:]
			p := doc.AddParagraph("")
			p.Style("List Bullet")
			addInlineRuns(p, text)
			continue
		}

		// Accumulate as paragraph text.
		paraLines = append(paraLines, line)
	}

	// Flush any remaining paragraph text.
	flushParagraph()

	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// inlineSpan represents a run of text with optional bold/italic formatting.
type inlineSpan struct {
	text   string
	bold   bool
	italic bool
}

// addInlineRuns parses inline Markdown (bold and italic) within a line and
// adds formatted runs to the paragraph.
func addInlineRuns(p *docxpkg.Paragraph, text string) {
	spans := parseInline(text)
	for _, s := range spans {
		if s.text == "" {
			continue
		}
		r := p.AddText(s.text)
		if s.bold {
			r.Bold(true)
		}
		if s.italic {
			r.Italic(true)
		}
	}
}

// parseInline parses a line of Markdown text into inline spans with bold/italic
// markers. It handles **bold**, _italic_, and *italic* (when not **bold**).
func parseInline(text string) []inlineSpan {
	var spans []inlineSpan
	i := 0
	n := len(text)

	for i < n {
		// Check for bold: **...**
		if i+1 < n && text[i] == '*' && text[i+1] == '*' {
			end := strings.Index(text[i+2:], "**")
			if end >= 0 {
				inner := text[i+2 : i+2+end]
				spans = append(spans, inlineSpan{text: inner, bold: true})
				i = i + 2 + end + 2
				continue
			}
		}

		// Check for italic with _..._
		if text[i] == '_' {
			end := strings.Index(text[i+1:], "_")
			if end >= 0 {
				inner := text[i+1 : i+1+end]
				spans = append(spans, inlineSpan{text: inner, italic: true})
				i = i + 1 + end + 1
				continue
			}
		}

		// Check for italic with *...* (single asterisk, not double)
		if text[i] == '*' && (i+1 >= n || text[i+1] != '*') {
			end := strings.Index(text[i+1:], "*")
			if end >= 0 {
				inner := text[i+1 : i+1+end]
				spans = append(spans, inlineSpan{text: inner, italic: true})
				i = i + 1 + end + 1
				continue
			}
		}

		// Plain character: find next special character.
		j := i + 1
		for j < n {
			if text[j] == '*' || text[j] == '_' {
				break
			}
			j++
		}
		spans = append(spans, inlineSpan{text: text[i:j]})
		i = j
	}

	return spans
}
