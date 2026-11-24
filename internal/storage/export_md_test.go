package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"personalcrm/internal/domain"
	"personalcrm/internal/storage"
)

func TestExportMarkdown(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer store.Close()

	p := &domain.Project{Title: "Personal CRM", Goal: "Ship portable TUI", Status: domain.ProjectActive, Priority: domain.PriorityHigh}
	_ = store.CreateProject(ctx, p)
	_ = store.CreateTask(ctx, &domain.Task{ProjectID: p.ID, Title: "Build CLI", Status: domain.TaskDone, Priority: domain.PriorityMedium})
	_ = store.CreateTask(ctx, &domain.Task{ProjectID: p.ID, Title: "Build TUI", Status: domain.TaskOpen, Priority: domain.PriorityHigh})
	_, _ = store.TagEntity(ctx, domain.EntityProject, p.ID, "golang")
	_ = store.AddProgressLog(ctx, &domain.ProgressLog{EntityType: domain.EntityProject, EntityID: p.ID, Content: "Completed storage layer"})

	_ = store.CreateIdea(ctx, &domain.Idea{Title: "Hydroponic Herbs", Status: domain.IdeaExploring, Source: "youtube"})
	_ = store.CreateHobby(ctx, &domain.Hobby{Name: "Cycling", Status: domain.HobbyActive, CurrentFocus: "Gravel routes"})
	_ = store.CreateNote(ctx, &domain.Note{Title: "Bubble Tea Tips", Content: "Use tea.ExecProcess for $EDITOR"})
	_ = store.CreateResource(ctx, &domain.Resource{Title: "Bubble Tea Repo", URLOrPath: "https://github.com/charmbracelet/bubbletea", ResourceType: domain.ResourceLink})
	_ = store.CreateRoutine(ctx, &domain.Routine{Title: "Daily Stretch", ScheduleType: domain.ScheduleDaily, Active: true})

	outDir := filepath.Join(t.TempDir(), "vault")
	if err := store.ExportMarkdown(ctx, outDir); err != nil {
		t.Fatalf("ExportMarkdown: %v", err)
	}

	// Verify project markdown contains YAML frontmatter, tasks checklist, and progress log.
	projFiles, err := filepath.Glob(filepath.Join(outDir, "projects", "*.md"))
	if err != nil || len(projFiles) != 1 {
		t.Fatalf("expected 1 project md file, got %v err=%v", projFiles, err)
	}
	rawProj, _ := os.ReadFile(projFiles[0])
	content := string(rawProj)
	for _, want := range []string{`title: "Personal CRM"`, `- [x] Build CLI`, `- [ ] Build TUI`, `Completed storage layer`, `"golang"`} {
		if !strings.Contains(content, want) {
			t.Fatalf("project markdown missing %q:\n%s", want, content)
		}
	}

	// Verify summary files exist.
	for _, rel := range []string{"resources.md", "routines.md"} {
		if _, err := os.Stat(filepath.Join(outDir, rel)); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}
}
