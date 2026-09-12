package converter

import (
	"fmt"
	"strings"

	"papertrail/internal/document"
	"papertrail/internal/scanner"

	"github.com/tsawler/tabula"
)

// PPTXConverter converts Microsoft PowerPoint .pptx files.
type PPTXConverter struct{}

func (PPTXConverter) Supports(extension string) bool {
	return hasExtension(extension, ".pptx")
}

func (PPTXConverter) Convert(file scanner.DocumentFile) (document.ExtractedContent, error) {
	doc := tabula.Open(file.AbsolutePath)
	defer doc.Close()

	text, _, err := doc.Text()
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("extract PPTX text: %w", err)
	}

	pageCount, err := doc.PageCount()
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("get PPTX slide count: %w", err)
	}

	return document.ExtractedContent{
		Text:      strings.TrimSpace(text),
		PageCount: pageCount,
	}, nil
}
