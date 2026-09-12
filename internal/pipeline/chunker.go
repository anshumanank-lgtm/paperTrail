package pipeline

import "strings"

const (
	chunkSize    = 300
	chunkOverlap = 50
)

// chunkText splits the input text into overlapping chunks of words, each with a maximum size of chunkSize
// and an overlap of chunkOverlap words between consecutive chunks. It returns a slice of strings,where each string is a chunk of text.
func chunkText(text string) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	if len(words) <= chunkSize {
		return []string{strings.Join(words, " ")}
	}

	step := chunkSize - chunkOverlap

	chunks := make(
		[]string,
		0,
		(len(words)+step-1)/step,
	)

	for start := 0; start < len(words); start += step {
		end := start + chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunks = append(
			chunks,
			strings.Join(words[start:end], " "),
		)

		if end == len(words) {
			break
		}
	}

	return chunks
}
