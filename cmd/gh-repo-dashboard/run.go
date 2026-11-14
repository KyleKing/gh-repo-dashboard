package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"

	"github.com/kyleking/gh-repo-dashboard/internal/cli"
	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

type runFlags struct {
	where  *string
	dryRun *bool
	due    *bool
	yes    *bool
	depth  *int
	mani   *string
}

func defineRunFlags(fs *flag.FlagSet) runFlags {
	return runFlags{
		where:  fs.String("where", "", "Also require this predicate expression to hold"),
		dryRun: fs.Bool("dry-run", false, "List the repos the task would run on and run nothing"),
		due:    fs.Bool("due", false, "Run only repos the task's every schedule calls for; with no task, every task"),
		yes:    fs.Bool("yes", false, "Skip the prompt a confirm task asks"),
		depth:  fs.Int("depth", 1, "Maximum directory depth to scan"),
		mani:   fs.String("mani", "", "Read the repo roster from a mani.yaml instead of scanning directories"),
	}
}

func newRunFlagSet(stderr io.Writer) (*flag.FlagSet, runFlags) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		//nolint:errcheck // usage text is best-effort stderr output
		fmt.Fprintf(stderr, "Usage: %[1]s run <task> [flags] [paths...]\n"+
			"       %[1]s run --due [flags] [paths...]\n\nFlags:\n", os.Args[0])
		fs.PrintDefaults()
	}

	return fs, defineRunFlags(fs)
}

// runCommand implements `run <task> [flags] [paths...]`. The task name may sit
// before or after the flags, which the flag package alone would not allow.
func runCommand(args []string, cfg config.Config, stdout, stderr io.Writer) error {
	fs, f := newRunFlagSet(stderr)

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if cfg.Depth > 0 && !flagSet(fs, "depth") {
		*f.depth = cfg.Depth
	}

	var task string
	if len(positional) > 0 && (!*f.due || hasTask(cfg.Tasks, positional[0])) {
		task, positional = positional[0], positional[1:]
	}

	roots, err := resolveRoster(*f.mani, positional, cfg.ScanPaths)
	if err != nil {
		return err
	}
	roots, err = absolutePaths(roots)
	if err != nil {
		return err
	}

	store, err := tasks.DefaultStore()
	if err != nil {
		return fmt.Errorf("opening run history: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err = cli.RunTasks(ctx, stdout, os.Stdin, roots, cli.RunOptions{
		Task: task, Due: *f.due, Where: *f.where, DryRun: *f.dryRun, Yes: *f.yes,
		MaxDepth: *f.depth, Tasks: cfg.Tasks, Store: store,
	})
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	return nil
}

// parseInterspersed parses flags that may follow positional arguments.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, fmt.Errorf("parsing run flags: %w", err)
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func flagSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) { set = set || f.Name == name })

	return set
}

func hasTask(all []config.Task, name string) bool {
	return slices.ContainsFunc(all, func(task config.Task) bool { return task.Name == name })
}

func exitWith(err error) {
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
