package converter

import (
	"fmt"
	"io"

	"papertrail/internal/document"
	"papertrail/internal/scanner"

	"github.com/ledongthuc/pdf"
)

// PDFConverter extracts embedded text from PDF files. It does not perform OCR.
type PDFConverter struct{}

func (PDFConverter) Supports(extension string) bool {
	return hasExtension(extension, ".pdf")
}

func (PDFConverter) Convert(file scanner.DocumentFile) (document.ExtractedContent, error) {
	pdfFile, reader, err := pdf.Open(file.AbsolutePath)
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("open PDF: %w", err)
	}
	defer pdfFile.Close()

	textReader, err := reader.GetPlainText()
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("extract PDF text: %w", err)
	}

	text, err := io.ReadAll(textReader)
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("read extracted PDF text: %w", err)
	}

	return document.ExtractedContent{
		Text:      string(text),
		PageCount: reader.NumPage(),
	}, nil
}
