package metadata

import (
	"context"

	"papertrail/internal/document"
)

type MetadataEngine interface {
	Extract(ctx context.Context, content document.ExtractedContent) (document.DocumentMetadata, error)
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}
