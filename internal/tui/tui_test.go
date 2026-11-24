package tui_test

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"personalcrm/internal/cli"
	"personalcrm/internal/config"
	"personalcrm/internal/domain"
	"personalcrm/internal/ext"
	"personalcrm/internal/storage"
	"personalcrm/internal/tui"
)

type mockPeoplePlugin struct {
	ext.BaseModule
	taskID    string
	projectID string
}

func (mockPeoplePlugin) ID() string           { return "people" }
func (mockPeoplePlugin) Name() string         { return "People" }
func (mockPeoplePlugin) Description() string  { return "Mock people plugin" }
func (mockPeoplePlugin) DefaultEnabled() bool { return true }
func (mockPeoplePlugin) TUIViewSpecs() []ext.TUIViewSpec {
	return []ext.TUIViewSpec{{ID: "people", Title: "People"}}
}
func (p mockPeoplePlugin) EntityBadges(_ context.Context, et domain.EntityType, id string) []string {
	if et == domain.EntityTask && id == p.taskID {
		return []string{"@claude"}
	}
	return nil
}
func (p mockPeoplePlugin) EntityDetailSections(_ context.Context, et domain.EntityType, id string) []ext.DetailSection {
	if et == domain.EntityProject && id == p.projectID {
		return []ext.DetailSection{{Title: "Customer & People", Lines: []string{"Customer: Acme Corp"}}}
	}
	return nil
}

func newTestApp(t *testing.T, reg *ext.Registry) (*cli.App, *storage.Store, *ext.EventBus) {
	t.Helper()
	ctx := context.Background()
	store, err := storage.OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cfg := config.Default()
	bus := ext.NewEventBus()
	if reg == nil {
		reg = ext.NewRegistry()
	}
	if err := reg.InitAll(ctx, store, bus, &cfg); err != nil {
		t.Fatalf("InitAll: %v", err)
	}
	return &cli.App{
		Version:  "test",
		Config:   cfg,
		Store:    store,
		Bus:      bus,
		Registry: reg,
	}, store, bus
}

func sendKey(m *tui.Model, s string) {
	var msg tea.KeyMsg
	switch s {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	m.Update(msg)
}

func typeString(m *tui.Model, text string) {
	for _, r := range text {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestTUIDashboardRefinementBannerAndRoutineToggle(t *testing.T) {
	ctx := context.Background()
	app, store, _ := newTestApp(t, nil)

	r := &domain.Routine{Title: "Morning Stretch", ScheduleType: domain.ScheduleDaily, Active: true}
	if err := store.CreateRoutine(ctx, r); err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}

	m := tui.New(ctx, app)
	// Tuesday 10:00 -> Sunday 18:00 refinement is overdue!
	tue := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	m.SetClock(func() time.Time { return tue })

	view := m.View()
	if !strings.Contains(view, "WEEKLY REFINEMENT DUE") {
		t.Fatalf("expected refinement banner on dashboard:\n%s", view)
	}
	if !strings.Contains(view, "[ ] Morning Stretch") {
		t.Fatalf("expected unchecked routine on dashboard:\n%s", view)
	}

	// Space checks off the due routine.
	sendKey(m, " ")
	viewAfter := m.View()
	if !strings.Contains(viewAfter, "[x] Morning Stretch") {
		t.Fatalf("expected checked routine after Space:\n%s", viewAfter)
	}

	// Snooze ('S') clears the refinement banner.
	sendKey(m, "S")
	viewSnoozed := m.View()
	if strings.Contains(viewSnoozed, "WEEKLY REFINEMENT DUE") {
		t.Fatalf("expected refinement banner cleared after snooze:\n%s", viewSnoozed)
	}
}

func TestTUIProjectsTasksAndPluginBadgesAndSections(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer store.Close()

	p := &domain.Project{Title: "PersonalCRM", Goal: "Ship v1", Status: domain.ProjectActive, Priority: domain.PriorityHigh}
	_ = store.CreateProject(ctx, p)
	task := &domain.Task{ProjectID: p.ID, Title: "Implement TUI", Status: domain.TaskOpen, Priority: domain.PriorityHigh}
	_ = store.CreateTask(ctx, task)

	reg := ext.NewRegistry()
	_ = reg.Register(mockPeoplePlugin{taskID: task.ID, projectID: p.ID})
	cfg := config.Default()
	bus := ext.NewEventBus()
	_ = reg.InitAll(ctx, store, bus, &cfg)

	app := &cli.App{Version: "test", Config: cfg, Store: store, Bus: bus, Registry: reg}
	m := tui.New(ctx, app)

	// Switch to Projects tab ('2').
	sendKey(m, "2")
	view := m.View()
	for _, want := range []string{"7 People", "@claude", "Customer & People", "Customer: Acme Corp", "Implement TUI"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in Projects view:\n%s", want, view)
		}
	}

	// Focus tasks ('right') and toggle task done (' ').
	sendKey(m, "right")
	sendKey(m, " ")
	updatedTask, _ := store.GetTask(ctx, task.ID)
	if updatedTask.Status != domain.TaskDone {
		t.Fatalf("expected task status done, got %s", updatedTask.Status)
	}

	// Switch back to projects ('left') and cycle status ('s': active -> waiting).
	sendKey(m, "left")
	sendKey(m, "s")
	updatedProj, _ := store.GetProject(ctx, p.ID)
	if updatedProj.Status != domain.ProjectWaiting {
		t.Fatalf("expected project status waiting, got %s", updatedProj.Status)
	}
}

func TestTUIIdeaPromoteModalFlow(t *testing.T) {
	ctx := context.Background()
	app, store, _ := newTestApp(t, nil)

	idea := &domain.Idea{Title: "Build Terrarium", Status: domain.IdeaReady}
	if err := store.CreateIdea(ctx, idea); err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	m := tui.New(ctx, app)
	sendKey(m, "3") // Ideas tab
	sendKey(m, "P") // Open Promote modal

	// Field 0: Title (pre-filled) -> Enter advances to Field 1 (Goal)
	sendKey(m, "enter")
	typeString(m, "Self-sustaining moss jar")
	// Field 1 -> Field 2 (Priority, pre-filled medium)
	sendKey(m, "enter")
	// Field 2 -> Field 3 (Initial tasks)
	sendKey(m, "enter")
	typeString(m, "Buy glass jar, Collect moss")
	// Submit modal
	sendKey(m, "enter")

	gotIdea, _ := store.GetIdea(ctx, idea.ID)
	if gotIdea.Status != domain.IdeaPromoted || gotIdea.PromotedProjectID == "" {
		t.Fatalf("expected idea promoted, got %+v", gotIdea)
	}
	tasks, _ := store.ListTasksByProject(ctx, gotIdea.PromotedProjectID, true)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 initial tasks on promoted project, got %d", len(tasks))
	}
}

func TestTUIGuidedWeeklyRefinementWizard(t *testing.T) {
	ctx := context.Background()
	app, store, bus := newTestApp(t, nil)

	p := &domain.Project{Title: "Write Docs", Status: domain.ProjectActive, Priority: domain.PriorityMedium}
	_ = store.CreateProject(ctx, p)
	_ = store.CreateTask(ctx, &domain.Task{ProjectID: p.ID, Title: "Draft README", Status: domain.TaskOpen, Priority: domain.PriorityMedium})
	_ = store.CreateIdea(ctx, &domain.Idea{Title: "Learn Rust", Status: domain.IdeaInbox})
	_ = store.CreateRoutine(ctx, &domain.Routine{Title: "Backup laptop", ScheduleType: domain.ScheduleWeekly, Active: true})

	var completedEvents int
	bus.Subscribe(domain.EventRefinementCompleted, func(_ context.Context, _ domain.Event) error {
		completedEvents++
		return nil
	})

	m := tui.New(ctx, app)
	tue := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	m.SetClock(func() time.Time { return tue })

	// Launch Refinement ('R').
	sendKey(m, "R")
	if !strings.Contains(m.View(), "1. Projects & Tasks") {
		t.Fatalf("expected refinement step 1:\n%s", m.View())
	}

	// Step 1: toggle task done + advance to Ideas.
	sendKey(m, " ")
	sendKey(m, "right")
	if !strings.Contains(m.View(), "Learn Rust") {
		t.Fatalf("expected idea in step 2:\n%s", m.View())
	}

	// Step 2: cycle idea status + advance to Routines.
	sendKey(m, "s")
	sendKey(m, "right")
	if !strings.Contains(m.View(), "Backup laptop") {
		t.Fatalf("expected routine in step 3:\n%s", m.View())
	}

	// Step 3: check off routine + advance to Summary.
	sendKey(m, " ")
	sendKey(m, "right")
	if !strings.Contains(m.View(), "Refinement Summary") {
		t.Fatalf("expected summary in step 4:\n%s", m.View())
	}

	// Step 4: complete Refinement ('enter').
	sendKey(m, "enter")

	if completedEvents != 1 {
		t.Fatalf("expected 1 EventRefinementCompleted, got %d", completedEvents)
	}
	if strings.Contains(m.View(), "WEEKLY REFINEMENT DUE") {
		t.Fatalf("expected refinement banner cleared after completion:\n%s", m.View())
	}
	runs, _ := store.ListRefinementRuns(ctx, 5)
	if len(runs) != 1 || runs[0].CompletedAt == nil || runs[0].ProjectsReviewed != 1 || runs[0].IdeasReviewed != 1 {
		t.Fatalf("unexpected refinement run persisted: %+v", runs)
	}
}
