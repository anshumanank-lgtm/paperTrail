// Package scanner discovers files and records their filesystem metadata.
package scanner

import (
	"fmt"
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

// Scan recursively discovers every file under folderPath. It does not read or
// parse file contents.
func Scan(folderPath string) ([]DocumentFile, error) {
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
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}

		extension := filepath.Ext(entry.Name())
		if _, ok := supportedExtensions[strings.ToLower(extension)]; !ok {
			return nil
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
