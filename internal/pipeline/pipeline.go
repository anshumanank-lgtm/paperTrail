package pipeline

import (
	"fmt"
	"sync"

	"papertrail/internal/converter"
	"papertrail/internal/document"
	"papertrail/internal/metadata"
	"papertrail/internal/scanner"
	"papertrail/logger"
)

// Pipeline represents a processing pipeline that scans a folder for files, converts them, and extracts metadata.
type Pipeline struct {
	converter *converter.Dispatcher
	metadata  metadata.MetadataEngine
	logger    *logger.Logger
	workers   int
}

// New creates a new Pipeline with the specified converter, metadata engine, and number of workers.
func New(
	converter *converter.Dispatcher,
	metadata metadata.MetadataEngine,
	logger *logger.Logger,
	workers int,
) *Pipeline {
	return &Pipeline{
		converter: converter,
		metadata:  metadata,
		logger:    logger,
		workers:   workers,
	}
}

// Run executes the pipeline on the specified folder, scanning for files, converting them, and extracting metadata.
func (p *Pipeline) Run(folderPath string) error {
	p.logger.Info("Scanning folder: %s", folderPath)
	files, err := scanner.Scan(folderPath)
	if err != nil {
		return err
	}
	// Create channels for jobs and results.
	jobs := make(chan scanner.DocumentFile)
	results := make(chan document.Document)

	var wg sync.WaitGroup
	// Start worker goroutines to process files concurrently.
	for i := 0; i < p.workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for file := range jobs {
				// Convert the file using the appropriate converter.
				content, err := p.converter.Convert(file)
				if err != nil {
					fmt.Printf("error: %s: %v\n", file.Filename, err)
					continue
				}
				// Extract metadata from the converted content.
				docMetadata := p.metadata.Extract(content)

				doc := document.Document{
					FileMetadata: document.FileMetadata{
						SourcePath: file.AbsolutePath,
						Filename:   file.Filename,
						Extension:  file.Extension,
						Size:       file.Size,
						ModifiedAt: file.ModifiedAt,
					},
					DocumentMetadata: docMetadata,
				}

				results <- doc
			}
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

	// Print the results as they are processed.
	for doc := range results {
		fmt.Printf("%+v\n", doc)
	}

	return nil
}
