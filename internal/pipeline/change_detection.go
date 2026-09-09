package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"papertrail/internal/scanner"
	"papertrail/internal/storage"
)

type indexAction int

const (
	indexNew indexAction = iota
	indexModified
)

type indexFile struct {
	File       scanner.DocumentFile
	Action     indexAction
	DocumentID uuid.UUID
}

func deduplicateFiles(files []scanner.DocumentFile) []scanner.DocumentFile {
	seen := make(map[string]struct{}, len(files))
	unique := make([]scanner.DocumentFile, 0, len(files))

	for _, file := range files {
		path, err := filepath.Abs(file.AbsolutePath)
		if err != nil {
			continue
		}
		path = filepath.Clean(path)
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		file.AbsolutePath = path
		unique = append(unique, file)
	}
	return unique
}

func (p *Pipeline) detectChanges(ctx context.Context, files []scanner.DocumentFile) ([]indexFile, error) {
	filesToProcess := make([]indexFile, 0)

	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		state, err := p.storage.GetDocumentIndexState(ctx, file.AbsolutePath)
		if err != nil {
			return nil, fmt.Errorf("get index state for %q: %w", file.AbsolutePath, err)
		}

		if state == nil {
			p.logger.Info("New file detected: %s", file.Filename)
			fingerprint, err := scanner.Fingerprint(ctx, file.AbsolutePath)
			if err != nil {
				return nil, fmt.Errorf("fingerprint new file %q: %w", file.AbsolutePath, err)
			}
			file.Fingerprint = fingerprint
			filesToProcess = append(filesToProcess, indexFile{File: file, Action: indexNew})
			continue
		}

		metadataUnchanged := state.Size == file.Size && state.ModifiedAt.Equal(file.ModifiedAt)
		if metadataUnchanged && state.IntelligenceStatus == storage.IntelligenceStatusReady {
			continue
		}

		if metadataUnchanged {
			// The source file is unchanged, but intelligence is incomplete. Re-run
			// intelligence from the source on the next pass.
			file.Fingerprint = state.Fingerprint
			filesToProcess = append(filesToProcess, indexFile{
				File:       file,
				Action:     indexModified,
				DocumentID: state.ID,
			})
			continue
		}

		fingerprint, err := scanner.Fingerprint(ctx, file.AbsolutePath)
		if err != nil {
			return nil, fmt.Errorf("fingerprint changed file %q: %w", file.AbsolutePath, err)
		}

		if fingerprint == state.Fingerprint && state.IntelligenceStatus == storage.IntelligenceStatusReady {
			continue
		}

		file.Fingerprint = fingerprint
		filesToProcess = append(filesToProcess, indexFile{
			File:       file,
			Action:     indexModified,
			DocumentID: state.ID,
		})
	}

	return filesToProcess, nil
}

func (p *Pipeline) deleteMissingDocuments(
	ctx context.Context,
	folderPaths []string,
	scannedFiles []scanner.DocumentFile,
) error {
	configuredFolders := make([]string, 0, len(folderPaths))
	for _, folderPath := range folderPaths {
		absolutePath, err := filepath.Abs(folderPath)
		if err != nil {
			return fmt.Errorf("resolve folder %q: %w", folderPath, err)
		}
		configuredFolders = append(configuredFolders, filepath.Clean(absolutePath))
	}

	scannedPaths := make(map[string]struct{}, len(scannedFiles))
	for _, file := range scannedFiles {
		absolutePath, err := filepath.Abs(file.AbsolutePath)
		if err != nil {
			return fmt.Errorf("resolve scanned path %q: %w", file.AbsolutePath, err)
		}
		scannedPaths[filepath.Clean(absolutePath)] = struct{}{}
	}

	documents, err := p.storage.ListDocuments(ctx)
	if err != nil {
		return fmt.Errorf("list indexed documents: %w", err)
	}

	for _, doc := range documents {
		if err := ctx.Err(); err != nil {
			return err
		}
		path := filepath.Clean(doc.FileMetadata.SourcePath)
		if !isUnderConfiguredFolder(path, configuredFolders) {
			continue
		}
		if _, exists := scannedPaths[path]; exists {
			continue
		}

		p.logger.Info("Deleting missing document: %s", doc.FileMetadata.Filename)
		if err := p.storage.DeleteDocument(ctx, doc.ID); err != nil {
			return fmt.Errorf("delete missing document %s: %w", doc.ID, err)
		}
	}
	return nil
}

func isUnderConfiguredFolder(path string, folders []string) bool {
	for _, folder := range folders {
		relative, err := filepath.Rel(folder, path)
		if err != nil {
			continue
		}
		if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			return true
		}
	}
	return false
}
