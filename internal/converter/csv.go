package converter

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"papertrail/internal/document"
	"papertrail/internal/scanner"
)

// CSVConverter converts CSV files into text suitable for indexing.
type CSVConverter struct{}

func (CSVConverter) Supports(extension string) bool {
	return hasExtension(extension, ".csv")
}

func (CSVConverter) Convert(file scanner.DocumentFile) (document.ExtractedContent, error) {
	f, err := os.Open(file.AbsolutePath)
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("open CSV: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)

	var builder strings.Builder

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return document.ExtractedContent{}, fmt.Errorf("read CSV: %w", err)
		}

		builder.WriteString(strings.Join(record, "\t"))
		builder.WriteByte('\n')
	}

	return document.ExtractedContent{
		Text: strings.TrimSpace(builder.String()),
	}, nil
}
