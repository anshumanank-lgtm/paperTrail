package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"

	"papertrail/internal/converter"
	"papertrail/internal/document"
	"papertrail/internal/metadata"
	"papertrail/internal/relationship"
	"papertrail/internal/scanner"
	"papertrail/internal/storage"
	"papertrail/logger"
)

type Pipeline struct {
	converter     *converter.Dispatcher
	metadata      metadata.MetadataEngine
	storage       storage.Storage
	relationships *relationship.Engine
	logger        *logger.Logger
	workers       int
}

type processedDocument struct {
	ID  uuid.UUID
	Doc document.Document
}

func New(
	converter *converter.Dispatcher,
	metadata metadata.MetadataEngine,
	storage storage.Storage,
	relationships *relationship.Engine,
	logger *logger.Logger,
	workers int,
) *Pipeline {
	return &Pipeline{
		converter:     converter,
		metadata:      metadata,
		storage:       storage,
		relationships: relationships,
		logger:        logger,
		workers:       workers,
	}
}

func (p *Pipeline) Run(folderPath string) error {
	ctx := context.Background()

	p.logger.Info("Starting pipeline")
	p.logger.Info("Scanning folder: %s", folderPath)

	files, err := scanner.Scan(folderPath)
	if err != nil {
		return fmt.Errorf("scan folder: %w", err)
	}

	p.logger.Info("Found %d supported files", len(files))

	processed, err := p.processFiles(ctx, files)
	if err != nil {
		return err
	}

	p.logger.Info("Processed %d documents", len(processed))

	if err := p.createRelationships(ctx, processed); err != nil {
		return err
	}

	if sqliteStorage, ok := p.storage.(*storage.SQLiteStorage); ok {
		if err := sqliteStorage.DumpTables(ctx); err != nil {
			return fmt.Errorf("dump SQLite tables: %w", err)
		}
	}

	p.logger.Info("Pipeline completed")

	return nil
}

func (p *Pipeline) processFiles(
	ctx context.Context,
	files []scanner.DocumentFile,
) ([]processedDocument, error) {
	p.logger.Info(
		"Starting worker pool with %d workers",
		p.workers,
	)

	jobs := make(chan scanner.DocumentFile)
	results := make(chan processedDocument)

	var wg sync.WaitGroup

	for i := 0; i < p.workers; i++ {
		workerID := i + 1

		wg.Add(1)

		go func() {
			defer wg.Done()

			p.logger.Info(
				"Worker %d started",
				workerID,
			)

			for file := range jobs {
				p.logger.Info(
					"Worker %d processing: %s",
					workerID,
					file.Filename,
				)

				result, err := p.processFile(ctx, file)
				if err != nil {
					p.logger.Error(
						"Worker %d failed processing %q: %v",
						workerID,
						file.Filename,
						err,
					)
					continue
				}

				results <- result

				p.logger.Info(
					"Worker %d completed: %s",
					workerID,
					file.Filename,
				)
			}

			p.logger.Info(
				"Worker %d stopped",
				workerID,
			)
		}()
	}

	go func() {
		defer close(jobs)

		for _, file := range files {
			jobs <- file
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var processed []processedDocument

	for result := range results {
		processed = append(processed, result)
	}

	return processed, nil
}

func (p *Pipeline) processFile(
	ctx context.Context,
	file scanner.DocumentFile,
) (processedDocument, error) {
	p.logger.Info(
		"Converting: %s",
		file.Filename,
	)

	content, err := p.converter.Convert(file)
	if err != nil {
		return processedDocument{}, fmt.Errorf(
			"convert: %w",
			err,
		)
	}

	p.logger.Info(
		"Extracting metadata: %s",
		file.Filename,
	)

	docMetadata := p.metadata.Extract(content)

	if docMetadata.Title == "" {
		docMetadata.Title = strings.TrimSuffix(
			file.Filename,
			filepath.Ext(file.Filename),
		)
	}

	doc := document.Document{
		ID: uuid.New(),
		FileMetadata: document.FileMetadata{
			SourcePath: file.AbsolutePath,
			Filename:   file.Filename,
			Extension:  file.Extension,
			Size:       file.Size,
			ModifiedAt: file.ModifiedAt,
		},
		DocumentMetadata: docMetadata,
	}

	p.logger.Info(
		"Storing document: %s",
		file.Filename,
	)

	documentID, err := p.storage.CreateDocument(
		ctx,
		doc,
		"",
	)
	if err != nil {
		return processedDocument{}, fmt.Errorf(
			"store document: %w",
			err,
		)
	}

	p.logger.Info(
		"Stored document %s with ID %s",
		file.Filename,
		documentID,
	)

	p.storeEntities(ctx, documentID, &doc)

	return processedDocument{
		ID:  documentID,
		Doc: doc,
	}, nil
}

func (p *Pipeline) storeEntities(
	ctx context.Context,
	documentID uuid.UUID,
	doc *document.Document,
) {
	for i := range doc.DocumentMetadata.Entities {
		entity := &doc.DocumentMetadata.Entities[i]

		entityID, err := p.storage.GetOrCreateEntity(
			ctx,
			*entity,
		)
		if err != nil {
			p.logger.Error(
				"Failed to store entity %q: %v",
				entity.Value,
				err,
			)
			continue
		}

		entity.ID = entityID

		if err := p.storage.AttachEntityToDocument(
			ctx,
			documentID,
			entityID,
			"",
		); err != nil {
			p.logger.Error(
				"Failed to attach entity %q: %v",
				entity.Value,
				err,
			)
			continue
		}
	}
}

func (p *Pipeline) createRelationships(
	ctx context.Context,
	documents []processedDocument,
) error {
	p.logger.Info(
		"Creating relationships between %d documents",
		len(documents),
	)

	for _, processed := range documents {
		p.logger.Info(
			"Processing relationships for document %s: %s",
			processed.ID,
			processed.Doc.FileMetadata.Filename,
		)

		if err := p.relationships.ProcessDocument(
			ctx,
			processed.ID,
		); err != nil {
			return fmt.Errorf(
				"create relationships for document %s: %w",
				processed.ID,
				err,
			)
		}
	}

	p.logger.Info("Relationship processing completed")

	return nil
}

func (p *Pipeline) outputDocuments(
	documents []processedDocument,
) {
	p.logger.Info(
		"Outputting %d documents",
		len(documents),
	)

	for _, processed := range documents {
		data, err := json.MarshalIndent(
			processed.Doc,
			"",
			"  ",
		)
		if err != nil {
			p.logger.Error(
				"Marshal document %s: %v",
				processed.ID,
				err,
			)
			continue
		}

		fmt.Println(string(data))
	}
}
