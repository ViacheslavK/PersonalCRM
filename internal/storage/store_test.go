package storage_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"personalcrm/internal/domain"
	"personalcrm/internal/storage"
)

func TestStoreProjectsTasksAndRefinementQuery(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer store.Close()

	pActive := &domain.Project{Title: "Build PersonalCRM", Goal: "Ship v1", Status: domain.ProjectActive, Priority: domain.PriorityHigh}
	pWaiting := &domain.Project{Title: "Home Server", Status: domain.ProjectWaiting, Priority: domain.PriorityMedium}
	pHold := &domain.Project{Title: "Learn Piano", Status: domain.ProjectOnHold, Priority: domain.PriorityLow}
	pDone := &domain.Project{Title: "Tax Filing", Status: domain.ProjectFinished, Priority: domain.PriorityHigh}
	pRej := &domain.Project{Title: "Old Idea", Status: domain.ProjectRejected, Priority: domain.PriorityLow}

	for _, p := range []*domain.Project{pActive, pWaiting, pHold, pDone, pRej} {
		if err := store.CreateProject(ctx, p); err != nil {
			t.Fatalf("CreateProject %s: %v", p.Title, err)
		}
	}

	eligible, err := store.ListRefinementProjects(ctx)
	if err != nil {
		t.Fatalf("ListRefinementProjects: %v", err)
	}
	if len(eligible) != 2 {
		t.Fatalf("expected 2 refinement-eligible projects (active+waiting), got %d", len(eligible))
	}

	// Create tasks under pActive.
	due := time.Now().UTC().Add(30 * time.Minute)
	t1 := &domain.Task{ProjectID: pActive.ID, Title: "Write schema", Status: domain.TaskOpen, Priority: domain.PriorityHigh, DueAt: &due, ReminderEnabled: true}
	t2 := &domain.Task{ProjectID: pActive.ID, Title: "Init repo", Status: domain.TaskDone, Priority: domain.PriorityLow}
	if err := store.CreateTask(ctx, t1); err != nil {
		t.Fatalf("CreateTask t1: %v", err)
	}
	if err := store.CreateTask(ctx, t2); err != nil {
		t.Fatalf("CreateTask t2: %v", err)
	}

	openTasks, err := store.ListTasksByProject(ctx, pActive.ID, false)
	if err != nil || len(openTasks) != 1 || openTasks[0].ID != t1.ID {
		t.Fatalf("expected 1 open task, got %+v err=%v", openTasks, err)
	}

	remTasks, err := store.ListDueReminderTasks(ctx)
	if err != nil || len(remTasks) != 1 || remTasks[0].ID != t1.ID {
		t.Fatalf("expected 1 reminder task, got %+v err=%v", remTasks, err)
	}

	// Deleting project cascades tasks and cleans polymorphic metadata.
	if _, err := store.TagEntity(ctx, domain.EntityProject, pActive.ID, "golang"); err != nil {
		t.Fatalf("TagEntity: %v", err)
	}
	if err := store.DeleteProject(ctx, pActive.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := store.GetTask(ctx, t1.ID); err != storage.ErrNotFound {
		t.Fatalf("expected task cascade delete, got err=%v", err)
	}
	tags, _ := store.ListTagsFor(ctx, domain.EntityProject, pActive.ID)
	if len(tags) != 0 {
		t.Fatalf("expected polymorphic tags cleaned on project delete, got %d", len(tags))
	}
}

func TestIdeaPromotionWithLinksNotesResourcesAndHobbies(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer store.Close()

	exc := 5
	idea := &domain.Idea{
		Title:       "DIY Split Keyboard",
		Description: "Ergonomic wireless Corne build",
		Status:      domain.IdeaExploring,
		Source:      "reddit",
		Excitement:  &exc,
	}
	if err := store.CreateIdea(ctx, idea); err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	hobby := &domain.Hobby{Name: "Electronics", Status: domain.HobbyActive, CurrentFocus: "Soldering"}
	if err := store.CreateHobby(ctx, hobby); err != nil {
		t.Fatalf("CreateHobby: %v", err)
	}

	note := &domain.Note{Title: "ZMK Firmware Notes", Content: "# ZMK\nUse nice!nano v2"}
	if err := store.CreateNote(ctx, note); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	res := &domain.Resource{Title: "Corne GitHub", URLOrPath: "https://github.com/foostan/crkbd", ResourceType: domain.ResourceLink}
	if err := store.CreateResource(ctx, res); err != nil {
		t.Fatalf("CreateResource: %v", err)
	}

	// Link note and resource to the idea, plus a tag.
	if err := store.AddLink(ctx, domain.EntityLink{
		SourceType: domain.EntityIdea, SourceID: idea.ID,
		TargetType: domain.EntityNote, TargetID: note.ID,
		RelationType: "research",
	}); err != nil {
		t.Fatalf("AddLink note: %v", err)
	}
	if err := store.AddLink(ctx, domain.EntityLink{
		SourceType: domain.EntityResource, SourceID: res.ID,
		TargetType: domain.EntityIdea, TargetID: idea.ID,
		RelationType: "reference",
	}); err != nil {
		t.Fatalf("AddLink resource: %v", err)
	}
	if _, err := store.TagEntity(ctx, domain.EntityIdea, idea.ID, "hardware"); err != nil {
		t.Fatalf("TagEntity: %v", err)
	}

	// Promote idea to project.
	proj, err := store.PromoteIdeaToProject(ctx, domain.PromoteIdeaInput{
		IdeaID:       idea.ID,
		Goal:         "Assemble and flash working split keyboard",
		Priority:     domain.PriorityHigh,
		InitialTasks: []string{"Order PCBs", "Buy switches", "Flash ZMK"},
		HobbyIDs:     []string{hobby.ID},
	})
	if err != nil {
		t.Fatalf("PromoteIdeaToProject: %v", err)
	}

	// Verify idea updated.
	updatedIdea, err := store.GetIdea(ctx, idea.ID)
	if err != nil {
		t.Fatalf("GetIdea: %v", err)
	}
	if updatedIdea.Status != domain.IdeaPromoted || updatedIdea.PromotedProjectID != proj.ID {
		t.Fatalf("idea not marked promoted properly: %+v", updatedIdea)
	}

	// Verify initial tasks created.
	tasks, err := store.ListTasksByProject(ctx, proj.ID, true)
	if err != nil || len(tasks) != 3 {
		t.Fatalf("expected 3 initial tasks, got %d err=%v", len(tasks), err)
	}

	// Verify tags carried over.
	projTags, err := store.ListTagsFor(ctx, domain.EntityProject, proj.ID)
	if err != nil || len(projTags) != 1 || projTags[0].Name != "hardware" {
		t.Fatalf("expected hardware tag carried over, got %+v err=%v", projTags, err)
	}

	// Verify links carried over (note + resource + hobby + inspired_by idea = 4 links).
	projLinks, err := store.ListLinksFor(ctx, domain.EntityProject, proj.ID)
	if err != nil || len(projLinks) != 4 {
		t.Fatalf("expected 4 links on promoted project, got %d (%+v) err=%v", len(projLinks), projLinks, err)
	}
}

func TestPluginNamespacedMigrationAndPolymorphicEntities(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer store.Close()

	// Simulate a future "people" plugin registering its own namespaced migration.
	peopleMigrations := []storage.Migration{
		{
			Version: 1,
			Name:    "create_ext_people",
			SQL: `CREATE TABLE ext_people (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				context TEXT NOT NULL DEFAULT ''
			);`,
		},
	}
	if err := store.ApplyMigrations(ctx, "ext:people", peopleMigrations); err != nil {
		t.Fatalf("ApplyMigrations ext:people: %v", err)
	}
	// Idempotent re-apply must succeed cleanly.
	if err := store.ApplyMigrations(ctx, "ext:people", peopleMigrations); err != nil {
		t.Fatalf("Re-apply ext:people: %v", err)
	}

	personID := domain.NewID()
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO ext_people (id, name, context) VALUES (?, ?, ?)`, personID, "Alex", "Woodworking meetup"); err != nil {
		t.Fatalf("insert ext_people: %v", err)
	}

	// Core Notes, Tags, and Progress Logs attach polymorphically to plugin entity "person"!
	personType := domain.EntityType("person")
	note := &domain.Note{Title: "Woodworking tips from Alex", Content: "Try Japanese pull saws"}
	if err := store.CreateNote(ctx, note); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if err := store.AddLink(ctx, domain.EntityLink{
		SourceType: personType, SourceID: personID,
		TargetType: domain.EntityNote, TargetID: note.ID,
	}); err != nil {
		t.Fatalf("AddLink person->note: %v", err)
	}
	if _, err := store.TagEntity(ctx, personType, personID, "mentor"); err != nil {
		t.Fatalf("TagEntity person: %v", err)
	}
	if err := store.AddProgressLog(ctx, &domain.ProgressLog{
		EntityType: personType,
		EntityID:   personID,
		Content:    "Caught up over coffee",
	}); err != nil {
		t.Fatalf("AddProgressLog person: %v", err)
	}

	links, _ := store.ListLinksFor(ctx, personType, personID)
	tags, _ := store.ListTagsFor(ctx, personType, personID)
	logs, _ := store.ListProgressLogs(ctx, personType, personID)
	if len(links) != 1 || len(tags) != 1 || len(logs) != 1 {
		t.Fatalf("expected polymorphic plugin relations (1,1,1), got (%d,%d,%d)", len(links), len(tags), len(logs))
	}
}

func TestRoutinesRefinementBackupAndExport(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "pcrm.db")

	store, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open file db: %v", err)
	}
	defer store.Close()

	// Routine with weekdays schedule.
	r := &domain.Routine{
		Title:        "Morning Walk",
		ScheduleType: domain.ScheduleWeekdays,
		ScheduleConfig: domain.RoutineScheduleConfig{
			Weekdays: []int{1, 3, 5},
		},
		Active:          true,
		ReminderEnabled: true,
		ReminderTime:    "08:00",
	}
	if err := store.CreateRoutine(ctx, r); err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}
	if err := store.SetRoutineCompleted(ctx, r.ID, "2026-11-23", time.Now().UTC()); err != nil {
		t.Fatalf("SetRoutineCompleted: %v", err)
	}
	gotR, err := store.GetRoutine(ctx, r.ID)
	if err != nil || gotR.CompletedCycleKey != "2026-11-23" {
		t.Fatalf("unexpected routine state: %+v err=%v", gotR, err)
	}

	// Refinement run + snooze state.
	if err := store.SnoozeRefinement(ctx, time.Now().UTC().Add(24*time.Hour)); err != nil {
		t.Fatalf("SnoozeRefinement: %v", err)
	}
	st, err := store.GetRefinementState(ctx)
	if err != nil || st.SnoozedUntil == nil {
		t.Fatalf("expected snoozed_until set, got %+v err=%v", st, err)
	}

	run, err := store.StartRefinementRun(ctx, "2026-11-22T18:00")
	if err != nil {
		t.Fatalf("StartRefinementRun: %v", err)
	}
	run.ProjectsReviewed = 3
	run.IdeasReviewed = 5
	run.RoutinesReviewed = 4
	if err := store.CompleteRefinementRun(ctx, &run); err != nil {
		t.Fatalf("CompleteRefinementRun: %v", err)
	}
	stAfter, err := store.GetRefinementState(ctx)
	if err != nil || stAfter.LastCompletedAt == nil || stAfter.SnoozedUntil != nil {
		t.Fatalf("expected completed set & snooze cleared, got %+v err=%v", stAfter, err)
	}

	// Online VACUUM INTO backup.
	backupPath := filepath.Join(dir, "backups", "pcrm-backup.db")
	if err := store.Backup(ctx, backupPath); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	backupStore, err := storage.Open(ctx, backupPath)
	if err != nil {
		t.Fatalf("Open backup db: %v", err)
	}
	defer backupStore.Close()
	backupRoutines, err := backupStore.ListRoutines(ctx, false)
	if err != nil || len(backupRoutines) != 1 {
		t.Fatalf("expected 1 routine in backup, got %d err=%v", len(backupRoutines), err)
	}

	// JSON export.
	raw, err := store.ExportJSON(ctx)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	var snap storage.ExportSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("unmarshal export json: %v", err)
	}
	if len(snap.Routines) != 1 || len(snap.RefinementRuns) != 1 {
		t.Fatalf("unexpected snapshot counts: routines=%d runs=%d", len(snap.Routines), len(snap.RefinementRuns))
	}
}
