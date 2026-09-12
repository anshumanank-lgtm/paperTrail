package pipeline

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"papertrail/internal/document"
	"papertrail/internal/storage"
)

const embeddingBatchSize = 32

var errStaleIntelligence = errors.New("stale intelligence result")

// processIntelligence handles the intelligence processing for a single document. It extracts metadata, generates embeddings,
// and updates the document's intelligence status in storage.
// It ensures that the document is still current before and after processing to avoid stale updates.
// It also replaces the document's entities with the extracted ones, filtering out any that should not be stored.
// It returns an error if any step fails, including if the document is no longer current or if there is a mismatch in the number of embeddings generated.
func (p *Pipeline) processIntelligence(
	ctx context.Context,
	job intelligenceJob,
) error {
	if err := p.ensureCurrentDocument(ctx, job); err != nil {
		return err
	}

	p.logger.Info(
		"Extracting intelligence: %s",
		job.FileName,
	)

	docMetadata, err := p.metadata.Extract(ctx, job.Content)
	if err != nil {
		return fmt.Errorf(
			"extract metadata: %w",
			err,
		)
	}

	if strings.TrimSpace(docMetadata.Title) == "" {
		docMetadata.Title = strings.TrimSuffix(
			job.FileName,
			filepath.Ext(job.FileName),
		)
	}

	if err := p.ensureCurrentDocument(ctx, job); err != nil {
		return err
	}

	// RAG indexing.
	chunks := chunkText(job.Content.Text)
	documentChunks := make([]storage.Chunk, 0, len(chunks))

	if len(chunks) > 0 {
		embeddings, err := p.embedChunks(ctx, chunks)
		if err != nil {
			return fmt.Errorf(
				"embed chunks: %w",
				err,
			)
		}

		if len(embeddings) != len(chunks) {
			return fmt.Errorf(
				"embedding count mismatch: got %d, want %d",
				len(embeddings),
				len(chunks),
			)
		}

		documentChunks = make([]storage.Chunk, len(chunks))

		for i := range chunks {
			documentChunks[i] = storage.Chunk{
				DocumentID: job.DocumentID,
				ChunkIndex: i,
				Text:       chunks[i],
				Embedding:  embeddings[i],
			}
		}

	}

	docMetadata.Entities = storedEntities(docMetadata.Entities)
	if err := p.storage.CommitDocumentIntelligence(
		ctx,
		job.DocumentID,
		job.Fingerprint,
		storage.IntelligenceResult{
			Metadata: docMetadata,
			Chunks:   documentChunks,
		},
	); err != nil {
		if errors.Is(err, storage.ErrStaleDocument) {
			p.logger.Info("Discarding stale intelligence result for %s", job.FileName)
			return errStaleIntelligence
		}
		return fmt.Errorf("commit intelligence: %w", err)
	}

	p.logger.Info(
		"Intelligence completed: %s",
		job.FileName,
	)

	return nil
}

// ensureCurrentDocument checks if the document is still current by comparing its fingerprint with the one in the job.
func (p *Pipeline) ensureCurrentDocument(ctx context.Context, job intelligenceJob) error {
	state, err := p.storage.GetDocumentIndexStateByID(ctx, job.DocumentID)
	if err != nil {
		return fmt.Errorf("check document state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("document no longer exists: %s", job.DocumentID)
	}
	if state.Fingerprint != job.Fingerprint {
		p.logger.Info("Discarding stale intelligence result for %s", job.FileName)
		return errStaleIntelligence
	}
	return nil
}

// replaceEntities replaces the entities of a document in storage with the provided extracted entities, filtering out any that should not be stored.
func storedEntities(extracted []document.Entity) []document.Entity {
	entities := make([]document.Entity, 0, len(extracted))
	for i := range extracted {
		entity := extracted[i]
		if shouldStoreEntity(entity.Type) {
			entities = append(entities, entity)
		}
	}

	return entities
}

// embedChunks generates embeddings for a list of text chunks in batches, returning a slice of embeddings corresponding to each chunk.
// It handles context cancellation and returns an error if the embedding process fails or if the number of embeddings does not match the number of chunks.
func (p *Pipeline) embedChunks(
	ctx context.Context,
	chunks []string,
) ([][]float32, error) {
	embeddings := make(
		[][]float32,
		0,
		len(chunks),
	)

	for start := 0; start < len(chunks); start += embeddingBatchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		end := start + embeddingBatchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch, err := p.metadata.Embed(
			ctx,
			chunks[start:end],
		)
		if err != nil {
			return nil, err
		}

		embeddings = append(
			embeddings,
			batch...,
		)
	}

	return embeddings, nil
}
