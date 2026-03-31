// Package pdf converts HTML documents to PDF using go-rod headless Chromium.
package pdf

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Converter converts HTML content to a PDF file.
type Converter interface {
	PDFFromHTML(ctx context.Context, html, outputPath string) error
}

// RodConverter implements Converter using go-rod headless Chromium.
type RodConverter struct {
	browser *rod.Browser
}

// NewRodConverter connects to a headless Chromium instance.
func NewRodConverter() (*RodConverter, error) {
	browser := rod.New()
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("pdf: connect browser: %w", err)
	}
	return &RodConverter{browser: browser}, nil
}

// Close shuts down the browser.
func (c *RodConverter) Close() error {
	return c.browser.Close()
}

// PDFFromHTML renders html as a page and saves an A4 PDF to outputPath.
func (c *RodConverter) PDFFromHTML(ctx context.Context, html, outputPath string) error {
	page, err := c.browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return fmt.Errorf("pdf: create page: %w", err)
	}
	defer page.Close()

	if err := page.SetDocumentContent(html); err != nil {
		return fmt.Errorf("pdf: set document content: %w", err)
	}

	f := func(v float64) *float64 { return &v }
	marginIn := f(1.5 / 2.54) // 1.5cm converted to inches

	stream, err := page.PDF(&proto.PagePrintToPDF{
		PrintBackground: true,
		PaperWidth:      f(8.27),  // A4 width in inches
		PaperHeight:     f(11.69), // A4 height in inches
		MarginTop:       marginIn,
		MarginBottom:    marginIn,
		MarginLeft:      marginIn,
		MarginRight:     marginIn,
	})
	if err != nil {
		return fmt.Errorf("pdf: print to pdf: %w", err)
	}
	defer stream.Close()

	pdfData, err := io.ReadAll(stream)
	if err != nil {
		return fmt.Errorf("pdf: read pdf stream: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("pdf: create output dir: %w", err)
	}
	if err := os.WriteFile(outputPath, pdfData, 0o644); err != nil {
		return fmt.Errorf("pdf: write file: %w", err)
	}
	return nil
}
