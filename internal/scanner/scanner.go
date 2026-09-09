package scanner

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DocumentFile describes a file discovered during a folder scan.
type DocumentFile struct {
	AbsolutePath string
	Filename     string
	Extension    string
	Size         int64
	ModifiedAt   time.Time
	Fingerprint  string
}

var supportedExtensions = map[string]struct{}{
	".pdf":  {},
	".txt":  {},
	".doc":  {},
	".docx": {},
	".xls":  {},
	".xlsx": {},
	".ppt":  {},
	".pptx": {},
}

// Scan recursively discovers every supported file under folderPath.
// It only reads filesystem metadata; file contents are not read.
func Scan(ctx context.Context, folderPath string) ([]DocumentFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	info, err := os.Stat(folderPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scan path is not a folder: %s", folderPath)
	}

	root, err := filepath.Abs(folderPath)
	if err != nil {
		return nil, err
	}

	var files []DocumentFile

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		extension := filepath.Ext(entry.Name())
		if _, ok := supportedExtensions[strings.ToLower(extension)]; !ok {
			return nil
		}

		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}

		files = append(files, DocumentFile{
			AbsolutePath: path,
			Filename:     entry.Name(),
			Extension:    extension,
			Size:         fileInfo.Size(),
			ModifiedAt:   fileInfo.ModTime(),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// Fingerprint calculates the SHA-256 fingerprint of a file.
// The file is read only when fingerprinting is explicitly requested.
func Fingerprint(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	buffer := make([]byte, 32*1024)

	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		n, err := file.Read(buffer)
		if n > 0 {
			if _, writeErr := hash.Write(buffer[:n]); writeErr != nil {
				return "", writeErr
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
