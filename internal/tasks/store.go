// Package tasks runs user-defined shell tasks across repos and records when
// each one last ran, so a front end can drive it through a progress callback.
package tasks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const stampMode = 0o600

// Stamp records a repo's last run of a task.
type Stamp struct {
	At          time.Time `json:"at"`
	Success     bool      `json:"success"`
	ExitCode    int       `json:"exit_code"`
	LastSuccess time.Time `json:"last_success"`
}

// Store keeps each (task, repo) pair's log and last-run stamp under one
// directory. Concurrent runs of different pairs never share a file.
type Store struct {
	dir string
}

// NewStore returns a Store rooted at dir.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// DefaultStore returns the Store under the user cache directory.
func DefaultStore() (*Store, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("resolving the user cache directory: %w", err)
	}

	return NewStore(filepath.Join(base, "gh-repo-dashboard", "tasks")), nil
}

func (s *Store) pairDir(task, repoPath string) string {
	sum := sha256.Sum256([]byte(repoPath))
	key := filepath.Base(repoPath) + "-" + hex.EncodeToString(sum[:4])

	return filepath.Join(s.dir, task, key)
}

// LogPath is where a (task, repo) run's combined output goes.
func (s *Store) LogPath(task, repoPath string) string {
	return filepath.Join(s.pairDir(task, repoPath), "run.log")
}

func (s *Store) stampPath(task, repoPath string) string {
	return filepath.Join(s.pairDir(task, repoPath), "last.json")
}

// LastStamp returns the recorded last run, if any.
func (s *Store) LastStamp(task, repoPath string) (Stamp, bool) {
	data, err := os.ReadFile(s.stampPath(task, repoPath))
	if err != nil {
		return Stamp{}, false
	}
	var stamp Stamp
	if err := json.Unmarshal(data, &stamp); err != nil {
		return Stamp{}, false
	}

	return stamp, true
}

// HistoryLimit is how many runs of a task the store keeps.
const HistoryLimit = 10

// Record summarizes one run of a task across its repos.
type Record struct {
	At       time.Time     `json:"at"`
	Duration time.Duration `json:"duration"`
	Repos    int           `json:"repos"`
	OK       int           `json:"ok"`
	Failed   int           `json:"failed"`
	Canceled int           `json:"canceled"`
}

func newRecord(at time.Time, results []Result) Record {
	rec := Record{At: at, Duration: time.Since(at), Repos: len(results)}
	for _, result := range results {
		switch result.Status {
		case StatusOK:
			rec.OK++
		case StatusFailed:
			rec.Failed++
		case StatusCanceled:
			rec.Canceled++
		}
	}

	return rec
}

func (s *Store) historyPath(task string) string {
	return filepath.Join(s.dir, task, "history.json")
}

// History returns the task's recorded runs, newest first, at most HistoryLimit.
func (s *Store) History(task string) []Record {
	data, err := os.ReadFile(s.historyPath(task))
	if err != nil {
		return nil
	}
	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil
	}

	slices.Reverse(records)

	return records
}

func (s *Store) appendRun(task string, rec Record) error {
	records := s.History(task)
	slices.Reverse(records)
	records = append(records, rec)
	records = records[max(len(records)-HistoryLimit, 0):]

	data, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("encoding history: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.historyPath(task)), logDirMode); err != nil {
		return fmt.Errorf("creating history directory: %w", err)
	}
	if err := os.WriteFile(s.historyPath(task), data, stampMode); err != nil {
		return fmt.Errorf("writing history: %w", err)
	}

	return nil
}

func (s *Store) record(task, repoPath string, exitCode int, at time.Time) error {
	stamp := Stamp{At: at, Success: exitCode == 0, ExitCode: exitCode}
	if previous, ok := s.LastStamp(task, repoPath); ok {
		stamp.LastSuccess = previous.LastSuccess
	}
	if stamp.Success {
		stamp.LastSuccess = at
	}

	data, err := json.Marshal(stamp)
	if err != nil {
		return fmt.Errorf("encoding stamp: %w", err)
	}
	if err := os.WriteFile(s.stampPath(task, repoPath), data, stampMode); err != nil {
		return fmt.Errorf("writing stamp: %w", err)
	}

	return nil
}
