package converter

import (
	"fmt"
	"strings"

	"papertrail/internal/document"
	"papertrail/internal/scanner"
)

// Converter converts one kind of discovered file into format-neutral content.
type Converter interface {
	Supports(extension string) bool
	Convert(file scanner.DocumentFile) (document.ExtractedContent, error)
}

// Dispatcher selects an appropriate Converter based on file extension.
type Dispatcher struct {
	converters []Converter
}

// New creates a Dispatcher from the provided converters.
func New(converters ...Converter) *Dispatcher {
	return &Dispatcher{converters: converters}
}

// Default returns a Dispatcher with the converters available in Papertrail 0.1.0.
func Default() *Dispatcher {
	return New(TextConverter{}, PDFConverter{})
}

// Convert converts one file using the converter registered for its extension.
func (d *Dispatcher) Convert(file scanner.DocumentFile) (document.ExtractedContent, error) {
	for _, converter := range d.converters {
		if converter.Supports(file.Extension) {
			content, err := converter.Convert(file)
			if err != nil {
				return document.ExtractedContent{}, fmt.Errorf(
					"convert %q: %w",
					file.AbsolutePath,
					err,
				)
			}
			return content, nil
		}
	}

	return document.ExtractedContent{}, fmt.Errorf(
		"convert %q: no converter for extension %q",
		file.AbsolutePath,
		file.Extension,
	)
}

// FileError identifies a file that could not be converted.
type FileError struct {
	SourcePath string
	Err        error
}

func (e FileError) Error() string {
	return e.Err.Error()
}

// ConvertAll converts every file independently.
func (d *Dispatcher) ConvertAll(
	files []scanner.DocumentFile,
) ([]document.ExtractedContent, []FileError) {
	contents := make([]document.ExtractedContent, 0, len(files))
	var errors []FileError

	for _, file := range files {
		content, err := d.Convert(file)
		if err != nil {
			errors = append(errors, FileError{
				SourcePath: file.AbsolutePath,
				Err:        err,
			})
			continue
		}

		contents = append(contents, content)
	}

	return contents, errors
}

func hasExtension(extension, expected string) bool {
	return strings.EqualFold(extension, expected)
}
