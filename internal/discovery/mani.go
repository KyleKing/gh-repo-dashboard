package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ManiFilename is the roster file looked for beside a scan path.
const ManiFilename = "mani.yaml"

// maniFile is the subset of mani's schema this tool reads: the project names,
// which are directories relative to the file.
type maniFile struct {
	Projects map[string]struct {
		Path string `yaml:"path"`
	} `yaml:"projects"`
}

// ManiPaths reads a mani.yaml roster and returns the repo directories it
// names, sorted, skipping any that do not exist on disk. A project's path
// defaults to its name, matching mani's own default.
func ManiPaths(maniPath string) ([]string, error) {
	data, err := os.ReadFile(maniPath) //nolint:gosec // the path is a user-supplied roster location
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", maniPath, err)
	}

	var file maniFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", maniPath, err)
	}

	base := filepath.Dir(maniPath)

	paths := make([]string, 0, len(file.Projects))
	for name, project := range file.Projects {
		dir := project.Path
		if dir == "" {
			dir = name
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(base, dir)
		}

		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}

		paths = append(paths, dir)
	}

	sort.Strings(paths)

	return paths, nil
}
