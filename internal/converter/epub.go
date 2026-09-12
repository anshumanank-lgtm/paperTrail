package converter

import (
	"fmt"
	"strings"

	"papertrail/internal/document"
	"papertrail/internal/scanner"

	"github.com/tsawler/tabula"
)

// EPUBConverter converts EPUB files.
type EPUBConverter struct{}

func (EPUBConverter) Supports(extension string) bool {
	return hasExtension(extension, ".epub")
}

func (EPUBConverter) Convert(file scanner.DocumentFile) (document.ExtractedContent, error) {
	doc := tabula.Open(file.AbsolutePath)
	defer doc.Close()

	text, _, err := doc.Text()
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("extract EPUB text: %w", err)
	}

	pageCount, err := doc.PageCount()
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("get EPUB page count: %w", err)
	}

	return document.ExtractedContent{
		Text:      strings.TrimSpace(text),
		PageCount: pageCount,
	}, nil
}
