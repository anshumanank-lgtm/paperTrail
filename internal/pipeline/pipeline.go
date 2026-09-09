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

type Pipeline struct {
	converter     *converter.Dispatcher
	metadata      metadata.MetadataEngine
	storage       storage.Storage
	relationships *relationship.Engine
	logger        *logger.Logger
	workers       int

	intelligenceQueue chan intelligenceJob
	pendingMu         sync.Mutex
	pending           map[uuid.UUID]string
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
		converter:         converter,
		metadata:          metadata,
		storage:           storage,
		relationships:     relationships,
		logger:            logger,
		workers:           workers,
		intelligenceQueue: make(chan intelligenceJob, 32),
		pending:           make(map[uuid.UUID]string),
	}
}

// Run owns the long-running pipeline lifecycle. A cycle scans and ingests files;
// intelligence processing runs independently through the bounded async queue.
func (p *Pipeline) Run(ctx context.Context, folderPaths []string, interval time.Duration) error {
	if len(folderPaths) == 0 {
		return fmt.Errorf("no folders configured")
	}
	if interval <= 0 {
		return fmt.Errorf("invalid scan interval: %s", interval)
	}

	p.logger.Info("Starting continuous pipeline")
	for _, folderPath := range folderPaths {
		p.logger.Info("Configured folder: %s", folderPath)
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var workerDone = make(chan struct{})
	go func() {
		p.runIntelligenceWorker(workerCtx)
		close(workerDone)
	}()

	defer func() { <-workerDone }()

	if err := p.runCycle(ctx, folderPaths); err != nil && !errors.Is(err, context.Canceled) {
		p.logger.Error("Pipeline cycle failed: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Pipeline shutdown requested")
			return nil
		case <-ticker.C:
			if err := p.runCycle(ctx, folderPaths); err != nil {
				if errors.Is(err, context.Canceled) {
					p.logger.Info("Pipeline shutdown requested")
					return nil
				}
				p.logger.Error("Pipeline cycle failed: %v", err)
			}
		}
	}
}

// runCycle owns one scan/index pass. It deliberately does not wait for ML
// intelligence; documents are persisted first and intelligence is queued.
func (p *Pipeline) runCycle(ctx context.Context, folderPaths []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.logger.Info("Starting pipeline cycle")

	files, err := p.scanFolders(ctx, folderPaths)
	if err != nil {
		return err
	}

	files = deduplicateFiles(files)
	p.logger.Info("Found %d supported files", len(files))

	if err := p.deleteMissingDocuments(ctx, folderPaths, files); err != nil {
		return fmt.Errorf("delete missing documents: %w", err)
	}

	filesToProcess, err := p.detectChanges(ctx, files)
	if err != nil {
		return fmt.Errorf("detect changes: %w", err)
	}

	p.logger.Info("Found %d new or modified files", len(filesToProcess))

	jobs, err := p.ingestFiles(ctx, filesToProcess)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if err := p.enqueueIntelligence(ctx, job); err != nil {
			return fmt.Errorf("enqueue intelligence for %s: %w", job.FileName, err)
		}
	}

	p.logger.Info("Pipeline cycle completed: %d documents queued for intelligence", len(jobs))
	return nil
}

func (p *Pipeline) scanFolders(ctx context.Context, folderPaths []string) ([]scanner.DocumentFile, error) {
	var files []scanner.DocumentFile
	for _, folderPath := range folderPaths {
		p.logger.Info("Scanning folder: %s", folderPath)
		folderFiles, err := scanner.Scan(ctx, folderPath)
		if err != nil {
			return nil, fmt.Errorf("scan folder %q: %w", folderPath, err)
		}
		files = append(files, folderFiles...)
	}
	return files, nil
}

func (p *Pipeline) enqueueIntelligence(ctx context.Context, job intelligenceJob) error {
	p.pendingMu.Lock()
	if fingerprint, exists := p.pending[job.DocumentID]; exists && fingerprint == job.Fingerprint {
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

func (p *Pipeline) releaseIntelligenceJob(job intelligenceJob) {
	p.pendingMu.Lock()
	defer p.pendingMu.Unlock()
	if fingerprint, exists := p.pending[job.DocumentID]; exists && fingerprint == job.Fingerprint {
		delete(p.pending, job.DocumentID)
	}
}

func (p *Pipeline) runIntelligenceWorker(ctx context.Context) {
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
				p.logger.Error("Intelligence failed for %q: %v", job.FileName, err)
			}
		}
	}
}
