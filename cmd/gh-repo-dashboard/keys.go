package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kyleking/aragonite/keyusage"

	"github.com/kyleking/gh-repo-dashboard/internal/app"
)

// seenUsage is what earlier sessions logged, nil when recording is off or the log cannot be read.
func seenUsage(rec *keyusage.Recorder) *keyusage.Usage {
	if rec == nil {
		return nil
	}

	u, err := keyusage.Load(keyusage.Dir(app.UsageApp), app.Keymap())
	if err != nil {
		return nil
	}

	return &u
}

func closeUsage(rec *keyusage.Recorder) {
	if err := rec.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
}

// commandTree is the CLI as keyusage sees it, walked from the flags the command defines.
func commandTree() keyusage.Command {
	return keyusage.Command{
		Name:  app.UsageApp,
		What:  "Browse and manage repos",
		Flags: flagNames(flag.CommandLine),
		Subs: []keyusage.Command{
			{Name: "keys", What: "Read the key usage log back against the keymap"},
			{
				Name:  "run",
				What:  "Run a task headlessly",
				Flags: flagNames(runFlagSet()),
			},
		},
	}
}

func runFlagSet() *flag.FlagSet {
	fs, _ := newRunFlagSet(io.Discard)

	return fs
}

func flagNames(fs *flag.FlagSet) []string {
	var names []string

	fs.VisitAll(func(f *flag.Flag) { names = append(names, "-"+f.Name, "--"+f.Name) })

	return names
}

// keysCommand implements `keys`: the usage log read back against the keymap.
func keysCommand(stdout io.Writer) error {
	dir := keyusage.Dir(app.UsageApp)
	if dir == "" {
		_, err := fmt.Fprintf(stdout, "no usage log: set %s or create ~/Sync\n", keyusage.EnvVar(app.UsageApp))

		return err //nolint:wrapcheck // plain write
	}

	km := app.Keymap()
	km[keyusage.CLI] = keyusage.Commands(commandTree())

	u, err := keyusage.Load(dir, km)
	if err != nil {
		return fmt.Errorf("reading the usage log: %w", err)
	}

	if err := keyusage.Write(stdout, u, km, time.Now().Add(-app.StaleAfter)); err != nil {
		return fmt.Errorf("writing the key report: %w", err)
	}

	return nil
}
