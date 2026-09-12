package converter

import (
	"fmt"
	"strings"

	"papertrail/internal/document"
	"papertrail/internal/scanner"

	"github.com/tsawler/tabula"
)

// XLSXConverter converts Microsoft Excel .xlsx files.
type XLSXConverter struct{}

func (XLSXConverter) Supports(extension string) bool {
	return hasExtension(extension, ".xlsx")
}

func (XLSXConverter) Convert(file scanner.DocumentFile) (document.ExtractedContent, error) {
	doc := tabula.Open(file.AbsolutePath)
	defer doc.Close()

	text, _, err := doc.Text()
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("extract XLSX text: %w", err)
	}

	pageCount, err := doc.PageCount()
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("get XLSX sheet count: %w", err)
	}

	return document.ExtractedContent{
		Text:      strings.TrimSpace(text),
		PageCount: pageCount,
	}, nil
}
