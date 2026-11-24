package editor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ResolveCommand determines the external editor command and arguments using:
// configEditor -> $VISUAL -> $EDITOR -> OS fallback (notepad.exe / nano / vi).
func ResolveCommand(configEditor string) []string {
	candidates := []string{
		strings.TrimSpace(configEditor),
		strings.TrimSpace(os.Getenv("VISUAL")),
		strings.TrimSpace(os.Getenv("EDITOR")),
	}
	for _, raw := range candidates {
		if raw != "" {
			if parts := splitCommandLine(raw); len(parts) > 0 {
				return parts
			}
		}
	}

	if runtime.GOOS == "windows" {
		return []string{"notepad.exe"}
	}
	if _, err := exec.LookPath("nano"); err == nil {
		return []string{"nano"}
	}
	return []string{"vi"}
}

// PrepareTempFile writes initialContent into a temporary Markdown file for external editing.
func PrepareTempFile(prefix, initialContent string) (string, error) {
	if prefix == "" {
		prefix = "pcrm-note-"
	}
	f, err := os.CreateTemp("", prefix+"*.md")
	if err != nil {
		return "", fmt.Errorf("create temp editor file: %w", err)
	}
	path := f.Name()
	if _, err := f.WriteString(initialContent); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write temp editor file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close temp editor file: %w", err)
	}
	return path, nil
}

// BuildCommand creates an *exec.Cmd ready to open path in the resolved external editor.
// Useful both for direct CLI invocation and for Bubble Tea's tea.ExecProcess.
func BuildCommand(ctx context.Context, configEditor, filePath string) *exec.Cmd {
	parts := ResolveCommand(configEditor)
	args := append(append([]string(nil), parts[1:]...), filePath)
	return exec.CommandContext(ctx, parts[0], args...)
}

// EditText opens initialContent in the external editor interactively, returning
// the edited content and whether it changed.
func EditText(ctx context.Context, configEditor, initialContent string) (string, bool, error) {
	tmpPath, err := PrepareTempFile("pcrm-note-", initialContent)
	if err != nil {
		return "", false, err
	}
	defer os.Remove(tmpPath)

	cmd := BuildCommand(ctx, configEditor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", false, fmt.Errorf("run editor %q: %w", strings.Join(ResolveCommand(configEditor), " "), err)
	}

	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", false, fmt.Errorf("read edited file: %w", err)
	}
	updated := strings.ReplaceAll(string(raw), "\r\n", "\n")
	orig := strings.ReplaceAll(initialContent, "\r\n", "\n")
	return updated, updated != orig, nil
}

// splitCommandLine splits a command string respecting single and double quotes
// so commands like `"C:\Program Files\VS Code\Code.exe" --wait` work on Windows.
func splitCommandLine(s string) []string {
	var parts []string
	var cur strings.Builder
	var quote rune

	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t':
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}
