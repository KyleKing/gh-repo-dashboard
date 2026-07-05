package models

import (
	"os"
	"path/filepath"
	"strings"
)

// notesFilenames lists candidate per-repo notes files, in priority order.
var notesFilenames = []string{".doing", "doing.md", "doing.txt", "TODO.md"}

const notesContentReadLimit = 64 * 1024

// DetectNotes finds the first matching notes file at repoPath's root and
// returns its name and first non-empty line (trimmed). Both are empty if no
// notes file exists; a read failure yields an empty first line, not an error,
// since notes detection is best-effort.
//
//nolint:gocritic // the (file, firstLine) result is documented above
func DetectNotes(repoPath string) (string, string) {
	for _, name := range notesFilenames {
		path := filepath.Join(repoPath, name)

		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		return name, firstNonEmptyLine(readCapped(path))
	}

	return "", ""
}

// ReadNotesFile reads the full content of a notes file previously identified
// by DetectNotes, capped at a modest size. A read failure yields "".
func ReadNotesFile(repoPath, notesFile string) string {
	if notesFile == "" {
		return ""
	}

	return readCapped(filepath.Join(repoPath, notesFile))
}

func readCapped(path string) string {
	data, err := os.ReadFile(path) //nolint:gosec // repo root + fixed filename
	if err != nil {
		return ""
	}

	if len(data) > notesContentReadLimit {
		data = data[:notesContentReadLimit]
	}

	return string(data)
}

func firstNonEmptyLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}

	return ""
}
