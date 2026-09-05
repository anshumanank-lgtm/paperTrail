package document

// ExtractedContent is format-neutral content produced by a document converter.
// Text is temporary input to metadata extraction and is not stored in Document.
type ExtractedContent struct {
	Text      string
	Headings  []string
	PageCount int
}
