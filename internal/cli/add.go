package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"personalcrm/internal/domain"
	"personalcrm/internal/editor"
)

func (a *App) runAdd(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		a.printAddUsage()
		return nil
	}

	kind := strings.ToLower(strings.TrimSpace(args[0]))
	switch kind {
	case "idea":
		return a.addIdea(ctx, args[1:])
	case "project", "proj":
		return a.addProject(ctx, args[1:])
	case "task":
		return a.addTask(ctx, args[1:])
	case "hobby":
		return a.addHobby(ctx, args[1:])
	case "routine":
		return a.addRoutine(ctx, args[1:])
	case "note":
		return a.addNote(ctx, args[1:])
	case "link", "resource":
		return a.addResource(ctx, args[1:])
	case "log":
		return a.addProgressLog(ctx, args[1:])
	default:
		// Bare shorthand: `pcrm add "Build a bird feeder" -t diy` -> Idea (inbox).
		return a.addIdea(ctx, args)
	}
}

func (a *App) addIdea(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add idea", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	desc := fs.String("d", "", "Description")
	source := fs.String("s", "", "Source (article, walk, conversation...)")
	status := fs.String("status", string(domain.IdeaInbox), "Status (inbox|exploring|ready|parked)")
	excitement := fs.Int("e", 0, "Excitement score 1..5")
	tags := fs.String("t", "", "Comma-separated tags")
	if err := fs.Parse(reorderFlags(args, fs)); err != nil {
		return err
	}

	title := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if title == "" {
		return errors.New("idea title is required: pcrm add idea \"Title\"")
	}

	idea := &domain.Idea{
		Title:       title,
		Description: strings.TrimSpace(*desc),
		Status:      domain.IdeaStatus(strings.TrimSpace(*status)),
		Source:      strings.TrimSpace(*source),
	}
	if *excitement > 0 {
		v := *excitement
		idea.Excitement = &v
	}

	if err := a.Store.CreateIdea(ctx, idea); err != nil {
		return err
	}
	if err := a.applyTags(ctx, domain.EntityIdea, idea.ID, *tags); err != nil {
		return err
	}
	a.publishEvent(ctx, domain.EventIdeaCreated, domain.EntityIdea, idea.ID)

	fmt.Fprintf(a.Stdout, "Added idea [%s] %s (%s)\n", shortID(idea.ID), idea.Title, idea.Status)
	return nil
}

func (a *App) addProject(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add project", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	desc := fs.String("d", "", "Description")
	goal := fs.String("goal", "", "Desired outcome / goal")
	priority := fs.String("priority", string(domain.PriorityMedium), "Priority (low|medium|high)")
	target := fs.String("target", "", "Target date (YYYY-MM-DD)")
	tags := fs.String("t", "", "Comma-separated tags")
	if err := fs.Parse(reorderFlags(args, fs)); err != nil {
		return err
	}

	title := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if title == "" {
		return errors.New("project title is required: pcrm add project \"Title\"")
	}

	now := time.Now().UTC()
	p := &domain.Project{
		Title:       title,
		Description: strings.TrimSpace(*desc),
		Goal:        strings.TrimSpace(*goal),
		Status:      domain.ProjectActive,
		Priority:    domain.Priority(strings.TrimSpace(*priority)),
		StartDate:   &now,
	}
	if strings.TrimSpace(*target) != "" {
		t, err := parseFlexibleDate(*target)
		if err != nil {
			return fmt.Errorf("invalid --target date: %w", err)
		}
		p.TargetDate = &t
	}

	if err := a.Store.CreateProject(ctx, p); err != nil {
		return err
	}
	if err := a.applyTags(ctx, domain.EntityProject, p.ID, *tags); err != nil {
		return err
	}
	a.publishEvent(ctx, domain.EventProjectCreated, domain.EntityProject, p.ID)

	fmt.Fprintf(a.Stdout, "Added project [%s] %s (%s, %s)\n", shortID(p.ID), p.Title, p.Status, p.Priority)
	return nil
}

func (a *App) addTask(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add task", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	projectRef := fs.String("project", "", "Project ID, prefix, or title substring")
	desc := fs.String("d", "", "Description")
	priority := fs.String("priority", string(domain.PriorityMedium), "Priority (low|medium|high)")
	due := fs.String("due", "", "Due date/time (YYYY-MM-DD or RFC3339)")
	remind := fs.Bool("remind", false, "Enable opt-in deadline reminder")
	if err := fs.Parse(reorderFlags(args, fs)); err != nil {
		return err
	}

	title := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if title == "" {
		return errors.New("task title is required: pcrm add task \"Title\" [--project <ref>]")
	}

	var projectID string
	if ref := strings.TrimSpace(*projectRef); ref != "" {
		id, err := a.resolveProjectRef(ctx, ref)
		if err != nil {
			return err
		}
		projectID = id
	}

	task := &domain.Task{
		ProjectID:       projectID,
		Title:           title,
		Description:     strings.TrimSpace(*desc),
		Status:          domain.TaskOpen,
		Priority:        domain.Priority(strings.TrimSpace(*priority)),
		ReminderEnabled: *remind,
	}
	if strings.TrimSpace(*due) != "" {
		t, err := parseFlexibleDate(*due)
		if err != nil {
			return fmt.Errorf("invalid --due date: %w", err)
		}
		task.DueAt = &t
	}

	if err := a.Store.CreateTask(ctx, task); err != nil {
		return err
	}
	a.publishEvent(ctx, domain.EventTaskCreated, domain.EntityTask, task.ID)

	fmt.Fprintf(a.Stdout, "Added task [%s] %s\n", shortID(task.ID), task.Title)
	return nil
}

func (a *App) addHobby(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add hobby", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	desc := fs.String("d", "", "Description")
	focus := fs.String("focus", "", "Current focus")
	tags := fs.String("t", "", "Comma-separated tags")
	if err := fs.Parse(reorderFlags(args, fs)); err != nil {
		return err
	}

	name := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if name == "" {
		return errors.New("hobby name is required: pcrm add hobby \"Name\"")
	}

	h := &domain.Hobby{
		Name:         name,
		Description:  strings.TrimSpace(*desc),
		CurrentFocus: strings.TrimSpace(*focus),
		Status:       domain.HobbyActive,
	}
	if err := a.Store.CreateHobby(ctx, h); err != nil {
		return err
	}
	if err := a.applyTags(ctx, domain.EntityHobby, h.ID, *tags); err != nil {
		return err
	}
	a.publishEvent(ctx, domain.EventHobbyCreated, domain.EntityHobby, h.ID)

	fmt.Fprintf(a.Stdout, "Added hobby [%s] %s (%s)\n", shortID(h.ID), h.Name, h.Status)
	return nil
}

func (a *App) addRoutine(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add routine", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	desc := fs.String("d", "", "Description")
	schedule := fs.String("schedule", string(domain.ScheduleDaily), "Schedule (daily|weekly|monthly|every_n_days|weekdays)")
	every := fs.Int("every", 0, "Interval in days (for every_n_days)")
	days := fs.String("days", "", "Comma-separated weekdays 0=Sun..6=Sat or mon,wed,fri (for weekdays)")
	remindTime := fs.String("remind", "", "Opt-in reminder time HH:MM (local)")
	if err := fs.Parse(reorderFlags(args, fs)); err != nil {
		return err
	}

	title := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if title == "" {
		return errors.New("routine title is required: pcrm add routine \"Title\" [--schedule daily]")
	}

	schedType := domain.ScheduleType(strings.ToLower(strings.TrimSpace(*schedule)))
	var schedCfg domain.RoutineScheduleConfig
	switch schedType {
	case domain.ScheduleEveryNDays:
		schedCfg.IntervalDays = *every
		schedCfg.AnchorDate = time.Now().Format("2006-01-02")
	case domain.ScheduleWeekdays:
		parsedDays, err := parseWeekdaysList(*days)
		if err != nil {
			return err
		}
		schedCfg.Weekdays = parsedDays
	}

	r := &domain.Routine{
		Title:           title,
		Description:     strings.TrimSpace(*desc),
		ScheduleType:    schedType,
		ScheduleConfig:  schedCfg,
		Active:          true,
		ReminderEnabled: strings.TrimSpace(*remindTime) != "",
		ReminderTime:    strings.TrimSpace(*remindTime),
	}
	if err := a.Store.CreateRoutine(ctx, r); err != nil {
		return err
	}
	a.publishEvent(ctx, domain.EventRoutineCreated, domain.EntityRoutine, r.ID)

	fmt.Fprintf(a.Stdout, "Added routine [%s] %s (%s)\n", shortID(r.ID), r.Title, r.ScheduleType)
	return nil
}

func (a *App) addNote(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add note", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	content := fs.String("c", "", "Markdown content")
	edit := fs.Bool("edit", false, "Open external $EDITOR to compose note content")
	source := fs.String("s", "", "Source")
	tags := fs.String("t", "", "Comma-separated tags")
	links := fs.String("link", "", "Comma-separated entity refs to link (e.g. idea:<id>,project:<id>)")
	if err := fs.Parse(reorderFlags(args, fs)); err != nil {
		return err
	}

	title := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if title == "" {
		return errors.New("note title is required: pcrm add note \"Title\"")
	}

	body := *content
	if *edit {
		edited, _, err := editor.EditText(ctx, a.Config.Editor, body)
		if err != nil {
			return err
		}
		body = edited
	}

	note := &domain.Note{
		Title:   title,
		Content: body,
		Source:  strings.TrimSpace(*source),
	}
	if err := a.Store.CreateNote(ctx, note); err != nil {
		return err
	}
	if err := a.applyTags(ctx, domain.EntityNote, note.ID, *tags); err != nil {
		return err
	}
	if err := a.applyLinks(ctx, domain.EntityNote, note.ID, *links); err != nil {
		return err
	}
	a.publishEvent(ctx, domain.EventNoteCreated, domain.EntityNote, note.ID)

	fmt.Fprintf(a.Stdout, "Added note [%s] %s\n", shortID(note.ID), note.Title)
	return nil
}

func (a *App) addResource(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add link", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	title := fs.String("title", "", "Resource title (defaults to URL/path)")
	resType := fs.String("type", string(domain.ResourceLink), "Type (link|book|course|tool|file|other)")
	status := fs.String("status", string(domain.ResourceReference), "Status (to_consume|in_progress|reference)")
	notes := fs.String("n", "", "Short notes")
	tags := fs.String("t", "", "Comma-separated tags")
	links := fs.String("link", "", "Comma-separated entity refs to link (e.g. idea:<id>)")
	if err := fs.Parse(reorderFlags(args, fs)); err != nil {
		return err
	}

	urlOrPath := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if urlOrPath == "" {
		return errors.New("URL or path is required: pcrm add link \"https://...\" [--title \"...\"]")
	}
	resTitle := strings.TrimSpace(*title)
	if resTitle == "" {
		resTitle = urlOrPath
	}

	res := &domain.Resource{
		Title:        resTitle,
		URLOrPath:    urlOrPath,
		ResourceType: domain.ResourceType(strings.TrimSpace(*resType)),
		Status:       domain.ResourceStatus(strings.TrimSpace(*status)),
		Notes:        strings.TrimSpace(*notes),
	}
	if err := a.Store.CreateResource(ctx, res); err != nil {
		return err
	}
	if err := a.applyTags(ctx, domain.EntityResource, res.ID, *tags); err != nil {
		return err
	}
	if err := a.applyLinks(ctx, domain.EntityResource, res.ID, *links); err != nil {
		return err
	}
	a.publishEvent(ctx, domain.EventResourceCreated, domain.EntityResource, res.ID)

	fmt.Fprintf(a.Stdout, "Added resource [%s] %s -> %s\n", shortID(res.ID), res.Title, res.URLOrPath)
	return nil
}

func (a *App) addProgressLog(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: pcrm add log <entity_type:id> \"Progress entry...\"")
	}
	entityType, entityID, err := parseEntityRef(args[0])
	if err != nil {
		return err
	}
	content := strings.TrimSpace(strings.Join(args[1:], " "))
	entry := &domain.ProgressLog{
		EntityType: entityType,
		EntityID:   entityID,
		Content:    content,
	}
	if err := a.Store.AddProgressLog(ctx, entry); err != nil {
		return err
	}
	a.publishEvent(ctx, domain.EventProgressLogged, entityType, entityID)

	fmt.Fprintf(a.Stdout, "Logged progress on %s:%s (%s)\n", entityType, shortID(entityID), entry.LoggedAt.Format("2006-01-02"))
	return nil
}

func (a *App) applyTags(ctx context.Context, entityType domain.EntityType, entityID, csv string) error {
	for _, raw := range strings.Split(csv, ",") {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if _, err := a.Store.TagEntity(ctx, entityType, entityID, tag); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) applyLinks(ctx context.Context, srcType domain.EntityType, srcID, csv string) error {
	for _, raw := range strings.Split(csv, ",") {
		ref := strings.TrimSpace(raw)
		if ref == "" {
			continue
		}
		tgtType, tgtID, err := parseEntityRef(ref)
		if err != nil {
			return err
		}
		if err := a.Store.AddLink(ctx, domain.EntityLink{
			SourceType:   srcType,
			SourceID:     srcID,
			TargetType:   tgtType,
			TargetID:     tgtID,
			RelationType: "related",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) resolveProjectRef(ctx context.Context, ref string) (string, error) {
	projects, err := a.Store.ListProjects(ctx, storageProjectAll())
	if err != nil {
		return "", err
	}
	refLower := strings.ToLower(ref)
	var matches []domain.Project
	for _, p := range projects {
		if p.ID == ref || strings.EqualFold(p.ID, ref) {
			return p.ID, nil
		}
		if strings.HasPrefix(strings.ToLower(p.ID), refLower) || strings.HasSuffix(strings.ToLower(p.ID), refLower) {
			matches = append(matches, p)
			continue
		}
		if strings.Contains(strings.ToLower(p.Title), refLower) {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no project matching %q", ref)
	case 1:
		return matches[0].ID, nil
	default:
		return "", fmt.Errorf("ambiguous project reference %q matched %d projects; use a more specific ID or title", ref, len(matches))
	}
}

func (a *App) publishEvent(ctx context.Context, evType domain.EventType, entType domain.EntityType, id string) {
	if a.Bus == nil {
		return
	}
	_ = a.Bus.Publish(ctx, domain.Event{
		Type:       evType,
		EntityType: entType,
		EntityID:   id,
		OccurredAt: time.Now().UTC(),
	})
}

func parseEntityRef(ref string) (domain.EntityType, string, error) {
	parts := strings.SplitN(strings.TrimSpace(ref), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("invalid entity reference %q (expected type:id, e.g. idea:01J...)", ref)
	}
	return domain.EntityType(strings.ToLower(strings.TrimSpace(parts[0]))), strings.TrimSpace(parts[1]), nil
}

func parseFlexibleDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			if layout == "2006-01-02" {
				t = time.Date(t.Year(), t.Month(), t.Day(), 18, 0, 0, 0, time.Local)
			}
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("expected YYYY-MM-DD, 'YYYY-MM-DD HH:MM', or RFC3339, got %q", raw)
}

func parseWeekdaysList(csv string) ([]int, error) {
	if strings.TrimSpace(csv) == "" {
		return nil, errors.New("--days is required for weekdays schedule (e.g. mon,wed,fri or 1,3,5)")
	}
	nameMap := map[string]int{
		"sun": 0, "sunday": 0,
		"mon": 1, "monday": 1,
		"tue": 2, "tuesday": 2,
		"wed": 3, "wednesday": 3,
		"thu": 4, "thursday": 4,
		"fri": 5, "friday": 5,
		"sat": 6, "saturday": 6,
	}
	seen := map[int]bool{}
	var out []int
	for _, tok := range strings.Split(csv, ",") {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if tok == "" {
			continue
		}
		var wd int
		if v, ok := nameMap[tok]; ok {
			wd = v
		} else if n, err := strconv.Atoi(tok); err == nil && n >= 0 && n <= 6 {
			wd = n
		} else {
			return nil, fmt.Errorf("invalid weekday %q", tok)
		}
		if !seen[wd] {
			seen[wd] = true
			out = append(out, wd)
		}
	}
	return out, nil
}

// reorderFlags moves flags before positional arguments so users can write
// `pcrm add idea "My title" -t tag1,tag2` naturally with Go's stdlib flag.FlagSet.
func reorderFlags(args []string, fs *flag.FlagSet) []string {
	boolFlags := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		type boolFlag interface{ IsBoolFlag() bool }
		if bf, ok := f.Value.(boolFlag); ok && bf.IsBoolFlag() {
			boolFlags[f.Name] = true
		}
	})

	var flagsPart, posPart []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			posPart = append(posPart, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			flagsPart = append(flagsPart, a)
			name := strings.TrimLeft(a, "-")
			if strings.Contains(name, "=") {
				continue
			}
			if !boolFlags[name] && i+1 < len(args) {
				flagsPart = append(flagsPart, args[i+1])
				i++
			}
		} else {
			posPart = append(posPart, a)
		}
	}
	return append(flagsPart, posPart...)
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[len(id)-8:]
}

func (a *App) printAddUsage() {
	fmt.Fprint(a.Stdout, `Quick Capture Commands:
  pcrm add "Idea title" [-d desc] [-t tag1,tag2] [-s source] [-e 1..5]
  pcrm add idea "Title" [-d desc] [--status inbox|exploring|ready] [-t tags]
  pcrm add project "Title" [--goal "..."] [--priority low|medium|high] [--target YYYY-MM-DD]
  pcrm add task "Title" [--project <id|title>] [--due YYYY-MM-DD] [--remind]
  pcrm add hobby "Name" [-d desc] [--focus "..."] [-t tags]
  pcrm add routine "Title" [--schedule daily|weekly|monthly|every_n_days|weekdays] [--days mon,wed,fri] [--every 3] [--remind HH:MM]
  pcrm add note "Title" [-c content] [--edit] [-t tags] [--link type:id]
  pcrm add link "https://..." [--title "..."] [--type link|book|course|tool|file|other] [--link type:id]
  pcrm add log <project|hobby>:<id> "Progress entry..."
`)
}
