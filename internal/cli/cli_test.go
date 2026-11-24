package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"personalcrm/internal/cli"
	"personalcrm/internal/ext"
	"personalcrm/internal/storage"
)

type demoPlugin struct {
	ext.BaseModule
}

func (demoPlugin) ID() string          { return "demo" }
func (demoPlugin) Name() string        { return "Demo Plugin" }
func (demoPlugin) Description() string { return "Demo CLI extension" }
func (demoPlugin) CLICommands() []ext.CLICommand {
	return []ext.CLICommand{
		{
			Name:    "hello-plugin",
			Summary: "Prints hello from plugin",
			Run: func(_ context.Context, _ []string, stdout, _ io.Writer) error {
				_, err := io.WriteString(stdout, "hello from demo plugin!\n")
				return err
			},
		},
	}
}

func runCLI(t *testing.T, dataDir string, reg *ext.Registry, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	fullArgs := append([]string{"--data-dir", dataDir}, args...)
	err := cli.Execute(context.Background(), fullArgs, cli.Options{
		Stdout:   &stdout,
		Stderr:   &stderr,
		Registry: reg,
	})
	return stdout.String(), err
}

func TestCLIQuickCaptureListExportBackupAndPlugins(t *testing.T) {
	dataDir := t.TempDir()

	// 1. Bare shorthand `pcrm add "..."` defaults to Idea (inbox) with trailing flags.
	out, err := runCLI(t, dataDir, nil, "add", "Build a bird feeder", "-t", "diy,woodworking", "-e", "4")
	if err != nil {
		t.Fatalf("bare add failed: %v", err)
	}
	if !strings.Contains(out, "Added idea") || !strings.Contains(out, "Build a bird feeder") {
		t.Fatalf("unexpected bare add output: %s", out)
	}

	// 2. Add project, then add task matching project by title substring!
	if _, err := runCLI(t, dataDir, nil, "add", "project", "Home Lab Upgrade", "--goal", "Quiet low-power server", "--priority", "high"); err != nil {
		t.Fatalf("add project failed: %v", err)
	}
	if _, err := runCLI(t, dataDir, nil, "add", "task", "Compare mini PCs", "--project", "Home Lab", "--due", "2026-12-01", "--remind"); err != nil {
		t.Fatalf("add task failed: %v", err)
	}

	// 3. Add hobby, routine (weekdays), note, and link.
	if _, err := runCLI(t, dataDir, nil, "add", "hobby", "Photography", "--focus", "Street portraits"); err != nil {
		t.Fatalf("add hobby failed: %v", err)
	}
	if _, err := runCLI(t, dataDir, nil, "add", "routine", "Morning Workout", "--schedule", "weekdays", "--days", "mon,wed,fri", "--remind", "07:30"); err != nil {
		t.Fatalf("add routine failed: %v", err)
	}
	if _, err := runCLI(t, dataDir, nil, "add", "note", "Server Power Notes", "-c", "Idle under 10W", "-t", "homelab"); err != nil {
		t.Fatalf("add note failed: %v", err)
	}
	if _, err := runCLI(t, dataDir, nil, "add", "link", "https://example.com/n100", "--title", "N100 Benchmarks", "--status", "to_consume"); err != nil {
		t.Fatalf("add link failed: %v", err)
	}

	// 4. List entities.
	out, err = runCLI(t, dataDir, nil, "list", "ideas")
	if err != nil || !strings.Contains(out, "Build a bird feeder") {
		t.Fatalf("list ideas unexpected: out=%q err=%v", out, err)
	}
	out, err = runCLI(t, dataDir, nil, "list", "routines")
	if err != nil || !strings.Contains(out, "Morning Workout") {
		t.Fatalf("list routines unexpected: out=%q err=%v", out, err)
	}

	// 5. Export JSON to stdout and verify snapshot contents.
	jsonOut, err := runCLI(t, dataDir, nil, "export", "--format", "json")
	if err != nil {
		t.Fatalf("export json failed: %v", err)
	}
	var snap storage.ExportSnapshot
	if err := json.Unmarshal([]byte(jsonOut), &snap); err != nil {
		t.Fatalf("unmarshal exported json: %v", err)
	}
	if len(snap.Projects) != 1 || len(snap.Tasks) != 1 || len(snap.Ideas) != 1 || len(snap.Hobbies) != 1 || len(snap.Routines) != 1 || len(snap.Notes) != 1 || len(snap.Resources) != 1 {
		t.Fatalf("unexpected export counts: p=%d t=%d i=%d h=%d r=%d n=%d res=%d",
			len(snap.Projects), len(snap.Tasks), len(snap.Ideas), len(snap.Hobbies), len(snap.Routines), len(snap.Notes), len(snap.Resources))
	}
	if snap.Tasks[0].ProjectID != snap.Projects[0].ID {
		t.Fatalf("expected task linked to resolved project ID %s, got %s", snap.Projects[0].ID, snap.Tasks[0].ProjectID)
	}

	// 6. Add progress log referencing project ID, then export Markdown and backup DB.
	if _, err := runCLI(t, dataDir, nil, "add", "log", "project:"+snap.Projects[0].ID, "Ordered RAM kit"); err != nil {
		t.Fatalf("add log failed: %v", err)
	}

	mdDir := filepath.Join(t.TempDir(), "md-export")
	if _, err := runCLI(t, dataDir, nil, "export", "--format", "markdown", "--out", mdDir); err != nil {
		t.Fatalf("export markdown failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mdDir, "routines.md")); err != nil {
		t.Fatalf("expected routines.md in export: %v", err)
	}

	backupFile := filepath.Join(t.TempDir(), "snap.db")
	if _, err := runCLI(t, dataDir, nil, "backup", "--out", backupFile); err != nil {
		t.Fatalf("backup failed: %v", err)
	}
	if info, err := os.Stat(backupFile); err != nil || info.Size() == 0 {
		t.Fatalf("expected non-empty backup file: err=%v", err)
	}

	// 7. Pluggable module enable/disable and CLI command dispatch.
	newReg := func() *ext.Registry {
		r := ext.NewRegistry()
		_ = r.Register(demoPlugin{})
		return r
	}

	// Initially disabled -> command rejected.
	if _, err := runCLI(t, dataDir, newReg(), "hello-plugin"); err == nil {
		t.Fatal("expected disabled plugin command to fail")
	}

	// Enable via `pcrm plugins enable demo`.
	if out, err = runCLI(t, dataDir, newReg(), "plugins", "enable", "demo"); err != nil || !strings.Contains(out, "enabled") {
		t.Fatalf("plugins enable failed: out=%q err=%v", out, err)
	}

	// Now plugin CLI command executes cleanly!
	if out, err = runCLI(t, dataDir, newReg(), "hello-plugin"); err != nil || !strings.Contains(out, "hello from demo plugin!") {
		t.Fatalf("enabled plugin command failed: out=%q err=%v", out, err)
	}
}
