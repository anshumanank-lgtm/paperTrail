package metadata

import (
	"papertrail/internal/document"
)

type MetadataEngine interface {
	Extract(content document.ExtractedContent) document.DocumentMetadata
}

func DefaultMetadataEngine() MetadataEngine {
	return &DefaultMetadataEngineImpl{}
}

type DefaultMetadataEngineImpl struct{}

func (d *DefaultMetadataEngineImpl) Extract(content document.ExtractedContent) document.DocumentMetadata {
	// Implement the logic to extract metadata from the content.
	// For now, we will return an empty DocumentMetadata struct.
	return document.DocumentMetadata{}
}
