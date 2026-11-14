package main

import (
	"fmt"
	"io"
	"time"

	"github.com/kyleking/aragonite/keyusage"

	"github.com/kyleking/gh-repo-dashboard/internal/app"
)

// openUsage is the recorder for this session and what earlier sessions logged.
// Both are nil when recording is off, and seen is nil when the log cannot be read.
func openUsage() (*keyusage.Recorder, *keyusage.Usage) {
	rec := keyusage.Open(app.UsageApp)
	if rec == nil {
		return nil, nil
	}

	u, err := keyusage.Load(keyusage.Dir(app.UsageApp), app.Keymap())
	if err != nil {
		return rec, nil
	}

	return rec, &u
}

// keysCommand implements `keys`: the usage log read back against the keymap.
func keysCommand(stdout io.Writer) error {
	dir := keyusage.Dir(app.UsageApp)
	if dir == "" {
		_, err := fmt.Fprintf(stdout, "no usage log: set %s or create ~/Sync\n", keyusage.EnvVar(app.UsageApp))

		return err //nolint:wrapcheck // plain write
	}

	km := app.Keymap()

	u, err := keyusage.Load(dir, km)
	if err != nil {
		return fmt.Errorf("reading the usage log: %w", err)
	}

	if err := keyusage.Write(stdout, u, km, time.Now().Add(-app.StaleAfter)); err != nil {
		return fmt.Errorf("writing the key report: %w", err)
	}

	return nil
}
