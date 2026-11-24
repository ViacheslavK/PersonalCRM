package editor_test

import (
	"os"
	"runtime"
	"testing"

	"personalcrm/internal/editor"
)

func TestResolveCommandPrecedenceAndQuoting(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	// Config takes highest precedence and preserves args & quoted paths.
	parts := editor.ResolveCommand(`"C:\Program Files\VS Code\Code.exe" --wait --new-window`)
	if len(parts) != 3 || parts[0] != `C:\Program Files\VS Code\Code.exe` || parts[1] != "--wait" || parts[2] != "--new-window" {
		t.Fatalf("unexpected quoted split: %#v", parts)
	}

	// Falls back to $VISUAL then $EDITOR.
	t.Setenv("EDITOR", "vim")
	t.Setenv("VISUAL", "code --wait")
	if p := editor.ResolveCommand(""); len(p) != 2 || p[0] != "code" || p[1] != "--wait" {
		t.Fatalf("expected $VISUAL fallback, got %#v", p)
	}

	t.Setenv("VISUAL", "")
	if p := editor.ResolveCommand(""); len(p) != 1 || p[0] != "vim" {
		t.Fatalf("expected $EDITOR fallback, got %#v", p)
	}

	// OS default when all empty.
	t.Setenv("EDITOR", "")
	p := editor.ResolveCommand("")
	if runtime.GOOS == "windows" && (len(p) != 1 || p[0] != "notepad.exe") {
		t.Fatalf("expected notepad.exe on Windows, got %#v", p)
	}
}

func TestPrepareTempFile(t *testing.T) {
	path, err := editor.PrepareTempFile("pcrm-test-", "# Hello\nBody")
	if err != nil {
		t.Fatalf("PrepareTempFile: %v", err)
	}
	defer os.Remove(path)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(raw) != "# Hello\nBody" {
		t.Fatalf("unexpected content: %q", string(raw))
	}
}
