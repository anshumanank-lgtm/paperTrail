package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"papertrail/internal/converter"
	"papertrail/internal/metadata"
	"papertrail/internal/relationship"
	"papertrail/internal/scanner"
	"papertrail/internal/storage"
	"papertrail/logger"
)

// Pipeline represents the document processing pipeline, responsible for scanning folders,
// detecting changes, ingesting files, and managing intelligence jobs.
type Pipeline struct {
	converter     *converter.Dispatcher
	metadata      metadata.MetadataEngine
	storage       storage.Storage
	relationships *relationship.Engine
	logger        *logger.Logger
	workers       int

	intelligenceQueue chan intelligenceJob
	workerDone        chan struct{}
	pendingMu         sync.Mutex
	pending           map[uuid.UUID]string
}

// New creates a new instance of the Pipeline with the provided components and configuration.
func New(
	converter *converter.Dispatcher,
	metadata metadata.MetadataEngine,
	storage storage.Storage,
	relationships *relationship.Engine,
	logger *logger.Logger,
	workers int,
) *Pipeline {
	return &Pipeline{
		converter:         converter,
		metadata:          metadata,
		storage:           storage,
		relationships:     relationships,
		logger:            logger,
		workers:           workers,
		intelligenceQueue: make(chan intelligenceJob, 32),
		workerDone:        make(chan struct{}),
		pending:           make(map[uuid.UUID]string),
	}
}

// Start begins the intelligence worker in a separate goroutine, allowing it to process
// intelligence jobs concurrently.
func (p *Pipeline) Start(ctx context.Context) {
	p.logger.Info("Starting intelligence worker")
	go func() {
		defer close(p.workerDone)
		p.runIntelligenceWorker(ctx)
	}()
}

func (p *Pipeline) Wait() {
	<-p.workerDone
}

// Scan scans the specified folder paths for supported document files, detects changes,
// and enqueues intelligence jobs for new or modified files.
// It provides progress updates through the provided progress callback function.
func (p *Pipeline) Scan(
	ctx context.Context,
	folderPaths []string,
	progress func(string),
) error {
	if len(folderPaths) == 0 {
		return fmt.Errorf("no folders configured")
	}

	if progress == nil {
		progress = func(string) {}
	}

	files, jobs, err := p.runCycle(
		ctx,
		folderPaths,
		progress,
	)
	if err != nil {
		return err
	}

	progress(
		fmt.Sprintf(
			"Found %d supported files",
			files,
		),
	)

	if len(jobs) > 0 {
		progress("Metadata extraction started")

		if err := p.waitForIntelligence(ctx); err != nil {
			return err
		}

		progress("Metadata extraction completed")
	}

	progress("Scanning completed")

	return nil
}

// runCycle performs a single cycle of the pipeline, scanning folders, detecting changes,
// and enqueuing intelligence jobs for new or modified files. It returns the total number of files scanned,
// the list of intelligence jobs created, and any error encountered during the process.
func (p *Pipeline) runCycle(
	ctx context.Context,
	folderPaths []string,
	progress func(string),
) (int, []intelligenceJob, error) {
	if err := ctx.Err(); err != nil {
		return 0, nil, err
	}

	p.logger.Info("Starting pipeline cycle")

	files, err := p.scanFolders(ctx, folderPaths)
	if err != nil {
		return 0, nil, err
	}

	files = deduplicateFiles(files)

	if err := p.deleteMissingDocuments(
		ctx,
		folderPaths,
		files,
	); err != nil {
		return 0, nil, fmt.Errorf("delete missing documents: %w", err)
	}

	filesToProcess, err := p.detectChanges(ctx, files)
	if err != nil {
		return 0, nil, fmt.Errorf("detect changes: %w", err)
	}

	p.logger.Info(
		"Found %d new or modified files",
		len(filesToProcess),
	)

	jobs, err := p.ingestFiles(ctx, filesToProcess)
	if err != nil {
		return 0, nil, err
	}

	for _, job := range jobs {
		if err := p.enqueueIntelligence(ctx, job); err != nil {
			return 0, nil, fmt.Errorf(
				"enqueue intelligence for %s: %w",
				job.FileName,
				err,
			)
		}
	}

	p.logger.Info(
		"Pipeline cycle completed: %d documents queued for intelligence",
		len(jobs),
	)

	return len(files), jobs, nil
}

// scanFolders scans the specified folder paths for supported document files and returns a list of detected files.
func (p *Pipeline) scanFolders(
	ctx context.Context,
	folderPaths []string,
) ([]scanner.DocumentFile, error) {
	var files []scanner.DocumentFile

	for _, folderPath := range folderPaths {
		p.logger.Info("Scanning folder: %s", folderPath)

		folderFiles, err := scanner.Scan(ctx, folderPath)
		if err != nil {
			return nil, fmt.Errorf(
				"scan folder %q: %w",
				folderPath,
				err,
			)
		}

		files = append(files, folderFiles...)
	}

	return files, nil
}

// waitForIntelligence waits for all pending intelligence jobs to complete.
// It checks the pending jobs map and blocks until there are no more pending jobs or the context is canceled.
func (p *Pipeline) waitForIntelligence(ctx context.Context) error {
	for {
		p.pendingMu.Lock()
		pending := len(p.pending)
		p.pendingMu.Unlock()

		if pending == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(50 * time.Millisecond):
		}
	}
}

// enqueueIntelligence enqueues an intelligence job for processing if it is not already pending.
// It checks the pending jobs map to avoid duplicate processing of the same document with the same fingerprint.
func (p *Pipeline) enqueueIntelligence(
	ctx context.Context,
	job intelligenceJob,
) error {
	p.pendingMu.Lock()

	if fingerprint, exists := p.pending[job.DocumentID]; exists &&
		fingerprint == job.Fingerprint {
		p.pendingMu.Unlock()
		return nil
	}

	p.pending[job.DocumentID] = job.Fingerprint
	p.pendingMu.Unlock()

	select {
	case p.intelligenceQueue <- job:
		return nil

	case <-ctx.Done():
		p.releaseIntelligenceJob(job)
		return ctx.Err()
	}
}

// releaseIntelligenceJob removes the specified intelligence job from the pending jobs map,
// allowing it to be processed again in the future if needed.
func (p *Pipeline) releaseIntelligenceJob(
	job intelligenceJob,
) {
	p.pendingMu.Lock()
	defer p.pendingMu.Unlock()

	if fingerprint, exists := p.pending[job.DocumentID]; exists &&
		fingerprint == job.Fingerprint {
		delete(p.pending, job.DocumentID)
	}
}

// runIntelligenceWorker continuously processes intelligence jobs from the queue.
// It runs in a separate goroutine and handles job processing, error logging,
// and graceful shutdown on context cancellation.
func (p *Pipeline) runIntelligenceWorker(
	ctx context.Context,
) {
	p.logger.Info("Intelligence worker started")
	defer p.logger.Info("Intelligence worker stopped")

	for {
		select {
		case <-ctx.Done():
			return

		case job := <-p.intelligenceQueue:
			err := p.processIntelligence(ctx, job)

			p.releaseIntelligenceJob(job)

			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				p.logger.Error(
					"Intelligence failed for %q: %v",
					job.FileName,
					err,
				)
			}
		}
	}
}
