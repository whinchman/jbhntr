package exporter

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

// isValidZip checks that the output starts with the PK ZIP magic bytes,
// which is the minimum requirement for a valid DOCX (ZIP-based) file.
func isValidZip(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04
}

// docxContent extracts all text from word/document.xml inside the DOCX bytes.
// DOCX is a ZIP archive; text content lives in word/document.xml (compressed).
func docxContent(t *testing.T, data []byte) string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("failed to open DOCX as ZIP: %v", err)
	}
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open word/document.xml: %v", err)
			}
			defer rc.Close()
			content, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("failed to read word/document.xml: %v", err)
			}
			return string(content)
		}
	}
	t.Fatal("word/document.xml not found in DOCX archive")
	return ""
}

func TestToDocx_EmptyInput(t *testing.T) {
	data, err := ToDocx("")
	if err != nil {
		t.Fatalf("ToDocx(\"\") returned error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("ToDocx(\"\") returned empty bytes")
	}
	if !isValidZip(data) {
		t.Fatalf("ToDocx(\"\") did not return a valid ZIP/DOCX: first bytes %x", data[:4])
	}
}

func TestToDocx_H1(t *testing.T) {
	data, err := ToDocx("# Hello World")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}
	xml := docxContent(t, data)
	if !bytes.Contains([]byte(xml), []byte("Hello World")) {
		t.Error("H1 text 'Hello World' not found in document XML")
	}
}

func TestToDocx_H2(t *testing.T) {
	data, err := ToDocx("## Section Two")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}
	xml := docxContent(t, data)
	if !bytes.Contains([]byte(xml), []byte("Section Two")) {
		t.Error("H2 text 'Section Two' not found in document XML")
	}
}

func TestToDocx_H3(t *testing.T) {
	data, err := ToDocx("### Sub Section")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}
	xml := docxContent(t, data)
	if !bytes.Contains([]byte(xml), []byte("Sub Section")) {
		t.Error("H3 text 'Sub Section' not found in document XML")
	}
}

func TestToDocx_PlainParagraph(t *testing.T) {
	data, err := ToDocx("This is a plain paragraph.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}
	xml := docxContent(t, data)
	if !bytes.Contains([]byte(xml), []byte("This is a plain paragraph.")) {
		t.Error("paragraph text not found in document XML")
	}
}

func TestToDocx_BoldRun(t *testing.T) {
	data, err := ToDocx("Some **bold** text here.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}
	xml := docxContent(t, data)
	if !bytes.Contains([]byte(xml), []byte("bold")) {
		t.Error("bold text 'bold' not found in document XML")
	}
}

func TestToDocx_ItalicRunUnderscore(t *testing.T) {
	data, err := ToDocx("Some _italic_ text here.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}
	xml := docxContent(t, data)
	if !bytes.Contains([]byte(xml), []byte("italic")) {
		t.Error("italic text 'italic' not found in document XML")
	}
}

func TestToDocx_ItalicRunAsterisk(t *testing.T) {
	data, err := ToDocx("Some *italic* text here.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}
	xml := docxContent(t, data)
	if !bytes.Contains([]byte(xml), []byte("italic")) {
		t.Error("italic text 'italic' not found in document XML")
	}
}

func TestToDocx_UnorderedListDash(t *testing.T) {
	data, err := ToDocx("- First item\n- Second item")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}
	xml := docxContent(t, data)
	if !bytes.Contains([]byte(xml), []byte("First item")) {
		t.Error("list item text 'First item' not found in document XML")
	}
	if !bytes.Contains([]byte(xml), []byte("Second item")) {
		t.Error("list item text 'Second item' not found in document XML")
	}
}

func TestToDocx_UnorderedListAsterisk(t *testing.T) {
	data, err := ToDocx("* Bullet one\n* Bullet two")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}
	xml := docxContent(t, data)
	if !bytes.Contains([]byte(xml), []byte("Bullet one")) {
		t.Error("list item text 'Bullet one' not found in document XML")
	}
}

func TestToDocx_MixedContent(t *testing.T) {
	md := `# My Resume

A brief summary of my background.

## Experience

- Worked at **Acme Corp** for 3 years
- Led the _platform team_

## Skills

Go, *Python*, Kubernetes`

	data, err := ToDocx(md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}

	xml := docxContent(t, data)
	expectedStrings := []string{
		"My Resume",
		"Experience",
		"Skills",
		"Acme Corp",
		"platform team",
		"Go",
		"Python",
		"Kubernetes",
	}
	for _, s := range expectedStrings {
		if !bytes.Contains([]byte(xml), []byte(s)) {
			t.Errorf("expected string %q not found in document XML", s)
		}
	}
}

func TestToDocx_UnknownMarkdownRendersAsPlainText(t *testing.T) {
	// Tables and code blocks should not panic and should render as plain text.
	md := "| Column1 | Column2 |\n|---------|---------|"
	data, err := ToDocx(md)
	if err != nil {
		t.Fatalf("unsupported Markdown should not cause an error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}
}

func TestToDocx_BlankLinesSeparateParagraphs(t *testing.T) {
	md := "First paragraph.\n\nSecond paragraph."
	data, err := ToDocx(md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isValidZip(data) {
		t.Fatal("result is not a valid DOCX")
	}
	xml := docxContent(t, data)
	if !bytes.Contains([]byte(xml), []byte("First paragraph.")) {
		t.Error("'First paragraph.' not found in document XML")
	}
	if !bytes.Contains([]byte(xml), []byte("Second paragraph.")) {
		t.Error("'Second paragraph.' not found in document XML")
	}
}

// Unit tests for parseInline (internal, in the same package).

func TestParseInline_PlainText(t *testing.T) {
	spans := parseInline("hello world")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].text != "hello world" || spans[0].bold || spans[0].italic {
		t.Errorf("unexpected span: %+v", spans[0])
	}
}

func TestParseInline_Bold(t *testing.T) {
	spans := parseInline("a **bold** word")
	found := false
	for _, s := range spans {
		if s.text == "bold" && s.bold {
			found = true
		}
	}
	if !found {
		t.Errorf("expected bold span with text 'bold', got: %+v", spans)
	}
}

func TestParseInline_ItalicUnderscore(t *testing.T) {
	spans := parseInline("an _italic_ word")
	found := false
	for _, s := range spans {
		if s.text == "italic" && s.italic {
			found = true
		}
	}
	if !found {
		t.Errorf("expected italic span with text 'italic', got: %+v", spans)
	}
}

func TestParseInline_ItalicAsterisk(t *testing.T) {
	spans := parseInline("an *italic* word")
	found := false
	for _, s := range spans {
		if s.text == "italic" && s.italic {
			found = true
		}
	}
	if !found {
		t.Errorf("expected italic span with text 'italic', got: %+v", spans)
	}
}

func TestParseInline_EmptyString(t *testing.T) {
	spans := parseInline("")
	if len(spans) != 0 {
		t.Errorf("expected 0 spans for empty string, got %d", len(spans))
	}
}
