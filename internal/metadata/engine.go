package metadata

import "papertrail/internal/document"

type MetadataEngine interface {
	Extract(content document.ExtractedContent) document.DocumentMetadata
}
