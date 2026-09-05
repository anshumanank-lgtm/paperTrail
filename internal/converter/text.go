package converter

import (
	"fmt"
	"os"

	"papertrail/internal/document"
	"papertrail/internal/scanner"
)

// TextConverter converts plain-text files.
type TextConverter struct{}

func (TextConverter) Supports(extension string) bool {
	return hasExtension(extension, ".txt")
}

func (TextConverter) Convert(file scanner.DocumentFile) (document.ExtractedContent, error) {
	text, err := extractText(file.AbsolutePath)
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("read text file: %w", err)
	}

	return document.ExtractedContent{
		Text: text,
	}, nil
}

func extractText(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(contents), nil
}
