package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"papertrail/internal/document"
)

type intelligenceJob struct {
	DocumentID  uuid.UUID
	Content     document.ExtractedContent
	Fingerprint string
	FileName    string
}

func (p *Pipeline) ingestFiles(ctx context.Context, files []indexFile) ([]intelligenceJob, error) {
	if len(files) == 0 {
		return nil, nil
	}

	workers := p.workers
	if workers <= 0 {
		workers = 1
	}
	if workers > len(files) {
		workers = len(files)
	}

	p.logger.Info("Starting ingestion worker pool with %d workers", workers)

	jobs := make(chan indexFile)
	results := make(chan intelligenceJob)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		workerID := i + 1
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.logger.Info("Ingestion worker %d started", workerID)
			defer p.logger.Info("Ingestion worker %d stopped", workerID)

			for {
				select {
				case <-ctx.Done():
					return
				case indexed, ok := <-jobs:
					if !ok {
						return
					}

					job, err := p.ingestFile(ctx, indexed)
					if err != nil {
						if errors.Is(err, context.Canceled) {
							return
						}
						p.logger.Error("Ingestion worker %d failed for %q: %v", workerID, indexed.File.Filename, err)
						continue
					}

					select {
					case results <- job:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, indexed := range files {
			select {
			case jobs <- indexed:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var intelligenceJobs []intelligenceJob
	for {
		select {
		case job, ok := <-results:
			if !ok {
				return intelligenceJobs, nil
			}
			intelligenceJobs = append(intelligenceJobs, job)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (p *Pipeline) ingestFile(ctx context.Context, indexed indexFile) (intelligenceJob, error) {
	if err := ctx.Err(); err != nil {
		return intelligenceJob{}, err
	}

	file := indexed.File
	documentID := indexed.DocumentID
	if documentID == uuid.Nil {
		documentID = uuid.New()
	}

	p.logger.Info("Converting: %s", file.Filename)
	content, err := p.converter.Convert(file)
	if err != nil {
		return intelligenceJob{}, fmt.Errorf("convert %q: %w", file.Filename, err)
	}

	// Basic persistence intentionally happens before GLiNER2. The document is
	// immediately visible to the rest of the application with intelligence pending.
	pending := document.Document{
		ID: documentID,
		FileMetadata: document.FileMetadata{
			SourcePath: file.AbsolutePath,
			Filename:   file.Filename,
			Extension:  file.Extension,
			Size:       file.Size,
			ModifiedAt: file.ModifiedAt,
			Author:     content.Author,
			Creator:    content.Creator,
		},
		DocumentMetadata: document.DocumentMetadata{
			DocumentType: document.DocumentTypeOther,
		},
	}

	p.logger.Info("Persisting basic document: %s", file.Filename)
	if indexed.Action == indexNew {
		if _, err := p.storage.CreateDocument(ctx, pending, file.Fingerprint); err != nil {
			return intelligenceJob{}, fmt.Errorf("create document: %w", err)
		}
	} else {
		if err := p.storage.DeleteDocumentRelationships(ctx, documentID); err != nil {
			return intelligenceJob{}, fmt.Errorf("clear document relationships: %w", err)
		}
		if err := p.storage.ReplaceDocumentEntities(ctx, documentID, nil); err != nil {
			return intelligenceJob{}, fmt.Errorf("clear document entities: %w", err)
		}
		if err := p.storage.UpdateDocument(ctx, documentID, pending, file.Fingerprint); err != nil {
			return intelligenceJob{}, fmt.Errorf("update document: %w", err)
		}
	}

	p.logger.Info("Persisted basic document %s with ID %s", file.Filename, documentID)
	return intelligenceJob{
		DocumentID:  documentID,
		Content:     content,
		Fingerprint: file.Fingerprint,
		FileName:    file.Filename,
	}, nil
}
