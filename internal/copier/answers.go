package copier

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// answersFilename is copier's default answers-file name, written to the root
// of a project generated from a template.
const answersFilename = ".copier-answers.yml"

// AnswersFile holds the fields of .copier-answers.yml this package uses.
type AnswersFile struct {
	SrcPath string `yaml:"_src_path"` //nolint:tagliatelle // fixed by copier's answers-file format
	Commit  string `yaml:"_commit"`   //nolint:tagliatelle // fixed by copier's answers-file format
}

// ReadAnswersFile reads and parses repoPath's .copier-answers.yml. It returns
// (nil, nil) when the file doesn't exist, since most repos aren't
// copier-generated.
func ReadAnswersFile(repoPath string) (*AnswersFile, error) {
	path := filepath.Join(repoPath, answersFilename)

	data, err := os.ReadFile(path) //nolint:gosec // path is a fixed filename under a caller-supplied repo dir
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil //nolint:nilnil // absence isn't an error: most repos aren't copier-generated
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", answersFilename, err)
	}

	var answers AnswersFile
	if err := yaml.Unmarshal(data, &answers); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", answersFilename, err)
	}

	return &answers, nil
}
