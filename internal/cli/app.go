package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"personalcrm/internal/config"
	"personalcrm/internal/ext"
	"personalcrm/internal/platform"
	"personalcrm/internal/storage"
)

// Version is the application semantic version (synchronized with root VERSION and
// overridable via -ldflags "-X personalcrm/internal/cli.Version=...").
var Version = "0.4.0"

// Options customizes CLI execution (primarily for testing and wiring TUI/Daemon hooks).
type Options struct {
	Stdout   io.Writer
	Stderr   io.Writer
	Registry *ext.Registry
	RunTUI   func(ctx context.Context, app *App) error
	RunDaemon func(ctx context.Context, app *App) error
}

// App bundles initialized services for CLI subcommands, TUI, and daemon modes.
type App struct {
	Version  string
	Paths    platform.Paths
	Config   config.Config
	Store    *storage.Store
	Bus      *ext.EventBus
	Registry *ext.Registry
	Stdout   io.Writer
	Stderr   io.Writer
}

// Execute parses global flags, initializes storage & enabled plugins when needed,
// and dispatches the requested subcommand.
func Execute(ctx context.Context, args []string, opts Options) error {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	dataDir, rest, err := parseGlobalFlags(args)
	if err != nil {
		return err
	}

	cmd := ""
	var subArgs []string
	if len(rest) > 0 {
		cmd = strings.ToLower(strings.TrimSpace(rest[0]))
		subArgs = rest[1:]
	}

	switch cmd {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "pcrm %s\n", Version)
		return nil
	case "help", "--help", "-h":
		printUsage(stdout, opts.Registry)
		return nil
	}

	paths, err := platform.ResolvePaths(dataDir)
	if err != nil {
		return fmt.Errorf("resolve paths: %w", err)
	}

	if cmd == "paths" {
		fmt.Fprintf(stdout, "Data dir:   %s\nConfig:     %s\nDatabase:   %s\nBackups:    %s\nPortable:   %v\n",
			paths.DataDir, paths.ConfigPath, paths.DBPath, paths.BackupDir, paths.Portable)
		return nil
	}

	if err := platform.EnsureDirs(paths); err != nil {
		return fmt.Errorf("ensure dirs: %w", err)
	}

	cfg, err := config.Load(paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	store, err := storage.Open(ctx, paths.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	bus := ext.NewEventBus()
	reg := opts.Registry
	if reg == nil {
		reg = ext.NewRegistry()
	}
	if err := reg.InitAll(ctx, store, bus, &cfg); err != nil {
		return fmt.Errorf("init plugins: %w", err)
	}

	app := &App{
		Version:  Version,
		Paths:    paths,
		Config:   cfg,
		Store:    store,
		Bus:      bus,
		Registry: reg,
		Stdout:   stdout,
		Stderr:   stderr,
	}

	switch cmd {
	case "", "tui":
		if opts.RunTUI != nil {
			return opts.RunTUI(ctx, app)
		}
		fmt.Fprintf(stdout, "PersonalCRM %s — TUI launches in Phase 4. Run `pcrm help` for CLI commands.\n", Version)
		return nil
	case "daemon":
		if opts.RunDaemon != nil {
			return opts.RunDaemon(ctx, app)
		}
		return errors.New("background tray daemon will be enabled in Phase 5")
	case "add":
		return app.runAdd(ctx, subArgs)
	case "list", "ls":
		return app.runList(ctx, subArgs)
	case "backup":
		return app.runBackup(ctx, subArgs)
	case "export":
		return app.runExport(ctx, subArgs)
	case "plugins":
		return app.runPlugins(ctx, subArgs)
	default:
		// Dispatch to enabled pluggable module CLI commands.
		for _, pc := range reg.CLICommands() {
			if strings.EqualFold(pc.Name, cmd) {
				return pc.Run(ctx, subArgs, stdout, stderr)
			}
		}
		return fmt.Errorf("unknown command %q (run `pcrm help` for usage)", cmd)
	}
}

func (a *App) runBackup(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	outPath := fs.String("out", "", "Destination SQLite backup file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dest := strings.TrimSpace(*outPath)
	if dest == "" {
		name := fmt.Sprintf("pcrm-%s.db", time.Now().Format("20060102-150405"))
		dest = filepath.Join(a.Paths.BackupDir, name)
	}
	if err := a.Store.Backup(ctx, dest); err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "Backup created: %s\n", dest)
	return nil
}

func (a *App) runExport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	format := fs.String("format", "json", "Export format: json or markdown")
	outPath := fs.String("out", "", "Output file (json) or directory (markdown)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		raw, err := a.Store.ExportJSON(ctx)
		if err != nil {
			return err
		}
		if dest := strings.TrimSpace(*outPath); dest != "" {
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, raw, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Exported JSON to: %s\n", dest)
			return nil
		}
		_, err = a.Stdout.Write(append(raw, '\n'))
		return err

	case "markdown", "md":
		dest := strings.TrimSpace(*outPath)
		if dest == "" {
			dest = filepath.Join(a.Paths.DataDir, "export-"+time.Now().Format("20060102-150405"))
		}
		if err := a.Store.ExportMarkdown(ctx, dest); err != nil {
			return err
		}
		fmt.Fprintf(a.Stdout, "Exported Markdown vault to: %s\n", dest)
		return nil

	default:
		return fmt.Errorf("unsupported export format %q (expected json or markdown)", *format)
	}
}

func (a *App) runPlugins(_ context.Context, args []string) error {
	action := "list"
	if len(args) > 0 {
		action = strings.ToLower(strings.TrimSpace(args[0]))
	}

	switch action {
	case "list", "ls":
		mods := a.Registry.List(a.Config)
		if len(mods) == 0 {
			fmt.Fprintln(a.Stdout, "No pluggable modules registered yet.")
			return nil
		}
		for _, m := range mods {
			state := "disabled"
			if m.Enabled {
				state = "enabled"
			}
			fmt.Fprintf(a.Stdout, "%-16s [%-8s] %s — %s\n", m.ID, state, m.Name, m.Description)
		}
		return nil

	case "enable", "disable":
		if len(args) < 2 {
			return fmt.Errorf("usage: pcrm plugins %s <module-id>", action)
		}
		id := strings.TrimSpace(args[1])
		if !a.Registry.Has(id) {
			return fmt.Errorf("unknown module %q (run `pcrm plugins list`)", id)
		}
		if a.Config.Plugins == nil {
			a.Config.Plugins = map[string]bool{}
		}
		a.Config.Plugins[id] = action == "enable"
		if err := config.Save(a.Paths.ConfigPath, a.Config); err != nil {
			return err
		}
		fmt.Fprintf(a.Stdout, "Plugin %q %sd (saved to %s)\n", id, action, a.Paths.ConfigPath)
		return nil

	default:
		return fmt.Errorf("unknown plugins subcommand %q (expected list, enable, or disable)", action)
	}
}

func parseGlobalFlags(args []string) (dataDir string, rest []string, err error) {
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--data-dir" {
			if i+1 >= len(args) {
				return "", nil, errors.New("--data-dir requires a path value")
			}
			dataDir = args[i+1]
			i += 2
			continue
		}
		if strings.HasPrefix(arg, "--data-dir=") {
			dataDir = strings.TrimPrefix(arg, "--data-dir=")
			i++
			continue
		}
		break
	}
	return dataDir, args[i:], nil
}

func printUsage(w io.Writer, reg *ext.Registry) {
	fmt.Fprintf(w, `PersonalCRM (pcrm %s) — Local-first Personal Projects, Ideas, Hobbies & Routines CRM

Usage:
  pcrm [--data-dir <path>] <command> [arguments]

Core Commands:
  tui                         Launch interactive TUI (default when no command given)
  add "<title>" [flags]       Quick-capture an Idea (shorthand for 'add idea')
  add idea|project|task|hobby|routine|note|link|log ...
  list [projects|ideas|hobbies|routines|notes|links]
  backup [--out <file.db>]    Create an online WAL-safe SQLite backup
  export [--format json|markdown] [--out <path>]
  plugins [list|enable|disable]
  paths                       Show resolved data/config/database paths
  version                     Print version
  help                        Show this help
`, Version)

	if reg != nil {
		if cmds := reg.CLICommands(); len(cmds) > 0 {
			fmt.Fprintln(w, "\nPlugin Commands:")
			for _, c := range cmds {
				fmt.Fprintf(w, "  %-26s %s\n", c.Name, c.Summary)
			}
		}
	}
}
