package pipeline

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"papertrail/internal/document"
	"papertrail/internal/storage"
)

func (p *Pipeline) processIntelligence(ctx context.Context, job intelligenceJob) error {
	if err := p.ensureCurrentDocument(ctx, job); err != nil {
		return err
	}

	p.logger.Info("Extracting intelligence: %s", job.FileName)
	docMetadata, err := p.metadata.Extract(job.Content)
	if err != nil {
		return fmt.Errorf("extract metadata: %w", err)
	}

	if strings.TrimSpace(docMetadata.Title) == "" {
		docMetadata.Title = strings.TrimSuffix(job.FileName, filepath.Ext(job.FileName))
	}

	if err := p.ensureCurrentDocument(ctx, job); err != nil {
		return err
	}

	if err := p.storage.UpdateDocumentIntelligence(ctx, job.DocumentID, docMetadata); err != nil {
		return fmt.Errorf("update intelligence metadata: %w", err)
	}

	if err := p.replaceEntities(ctx, job.DocumentID, docMetadata.Entities); err != nil {
		return err
	}

	if err := p.relationships.ProcessDocument(ctx, job.DocumentID); err != nil {
		return fmt.Errorf("create relationships: %w", err)
	}

	if err := p.ensureCurrentDocument(ctx, job); err != nil {
		return err
	}
	if err := p.storage.SetDocumentIntelligenceStatus(ctx, job.DocumentID, storage.IntelligenceStatusReady); err != nil {
		return fmt.Errorf("mark document ready: %w", err)
	}

	p.logger.Info("Intelligence completed: %s", job.FileName)
	return nil
}

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

func (p *Pipeline) replaceEntities(ctx context.Context, documentID uuid.UUID, extracted []document.Entity) error {
	entities := make([]document.Entity, 0, len(extracted))
	for i := range extracted {
		entity := extracted[i]
		if shouldStoreEntity(entity.Type) {
			entities = append(entities, entity)
		}
	}

	if err := p.storage.ReplaceDocumentEntities(ctx, documentID, entities); err != nil {
		return fmt.Errorf("replace document entities: %w", err)
	}
	return nil
}

var errStaleIntelligence = errors.New("stale intelligence result")
