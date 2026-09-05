// Package cli contains Papertrail's command-line wiring.
package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"papertrail/internal/converter"
	"papertrail/internal/scanner"
)

// Run scans and converts the supplied folder, writing converted documents as JSON.
// It returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: papertrail <folder-path>")
		return 2
	}

	files, err := scanner.Scan(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	documents, conversionErrors := converter.Default().ConvertAll(files)
	for _, conversionError := range conversionErrors {
		fmt.Fprintln(stderr, conversionError)
	}

	if err := json.NewEncoder(stdout).Encode(documents); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
