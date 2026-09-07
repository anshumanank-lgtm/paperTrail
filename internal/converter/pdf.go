package converter

import (
	"context"
	"fmt"
	"os"

	"papertrail/internal/document"
	"papertrail/internal/scanner"

	"github.com/giraffesyo/pdf"
)

// PDFConverter extracts embedded text and metadata from PDF files.
// It does not perform OCR.
type PDFConverter struct{}

func (PDFConverter) Supports(extension string) bool {
	return hasExtension(extension, ".pdf")
}

func (PDFConverter) Convert(file scanner.DocumentFile) (document.ExtractedContent, error) {
	f, err := os.Open(file.AbsolutePath)
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("open PDF: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("stat PDF: %w", err)
	}

	pdfDocument, err := pdf.ExtractWithOptions(
		context.Background(),
		f,
		info.Size(),
		pdf.Options{
			IncludeMetadata: true,
		},
	)
	if err != nil {
		return document.ExtractedContent{}, fmt.Errorf("extract PDF: %w", err)
	}

	return document.ExtractedContent{
		Text:      pdfDocument.Text(),
		PageCount: pdfDocument.PageCount,
		Author:    pdfDocument.Metadata.Author,
		Creator:   pdfDocument.Metadata.Creator,
	}, nil
}
