package converter

import (
	"fmt"
	"strings"

	"papertrail/internal/document"
	"papertrail/internal/scanner"

	"github.com/tsawler/tabula"
)

// DOCXConverter converts Microsoft Word .docx files.
type DOCXConverter struct{}

func (DOCXConverter) Supports(extension string) bool {
	return hasExtension(extension, ".docx")
}

func (DOCXConverter) Convert(file scanner.DocumentFile) (document.ExtractedContent, error) {
	doc := tabula.Open(file.AbsolutePath)
	defer doc.Close()

	text, _, err := doc.Text()
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("extract DOCX text: %w", err)
	}

	pageCount, err := doc.PageCount()
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("get DOCX page count: %w", err)
	}

	return document.ExtractedContent{
		Text:      strings.TrimSpace(text),
		PageCount: pageCount,
	}, nil
}
