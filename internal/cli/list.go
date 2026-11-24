package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"personalcrm/internal/domain"
	"personalcrm/internal/storage"
)

func storageProjectAll() storage.ProjectFilter {
	return storage.ProjectFilter{}
}

func (a *App) runList(ctx context.Context, args []string) error {
	target := "projects"
	if len(args) > 0 {
		target = strings.ToLower(strings.TrimSpace(args[0]))
	}

	switch target {
	case "projects", "project", "p":
		items, err := a.Store.ListProjects(ctx, storage.ProjectFilter{})
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Fprintln(a.Stdout, "No projects yet.")
			return nil
		}
		for _, p := range items {
			fmt.Fprintf(a.Stdout, "[%s] %-9s %-6s %s\n", shortID(p.ID), p.Status, p.Priority, p.Title)
		}
		return nil

	case "ideas", "idea", "inbox", "i":
		items, err := a.Store.ListIdeas(ctx)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Fprintln(a.Stdout, "No ideas yet.")
			return nil
		}
		for _, it := range items {
			fmt.Fprintf(a.Stdout, "[%s] %-10s %s\n", shortID(it.ID), it.Status, it.Title)
		}
		return nil

	case "hobbies", "hobby", "h":
		items, err := a.Store.ListHobbies(ctx)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Fprintln(a.Stdout, "No hobbies yet.")
			return nil
		}
		for _, h := range items {
			fmt.Fprintf(a.Stdout, "[%s] %-8s %s\n", shortID(h.ID), h.Status, h.Name)
		}
		return nil

	case "routines", "routine", "r":
		items, err := a.Store.ListRoutines(ctx, false)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Fprintln(a.Stdout, "No routines yet.")
			return nil
		}
		now := time.Now()
		for _, r := range items {
			done, info, _ := domain.IsCompletedForCycle(r, now)
			mark := " "
			if !r.Active {
				mark = "-"
			} else if !info.IsDue {
				mark = "."
			} else if done {
				mark = "x"
			}
			fmt.Fprintf(a.Stdout, "[%s] [%s] %-12s %s\n", shortID(r.ID), mark, r.ScheduleType, r.Title)
		}
		return nil

	case "notes", "note", "n":
		items, err := a.Store.ListNotes(ctx, "")
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Fprintln(a.Stdout, "No notes yet.")
			return nil
		}
		for _, n := range items {
			fmt.Fprintf(a.Stdout, "[%s] %s (%s)\n", shortID(n.ID), n.Title, n.UpdatedAt.Format("2006-01-02"))
		}
		return nil

	case "links", "resources", "link", "resource":
		items, err := a.Store.ListResources(ctx, "")
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Fprintln(a.Stdout, "No resources/links yet.")
			return nil
		}
		for _, r := range items {
			fmt.Fprintf(a.Stdout, "[%s] %-11s %s — %s\n", shortID(r.ID), r.Status, r.Title, r.URLOrPath)
		}
		return nil

	default:
		return fmt.Errorf("unknown list target %q (expected projects, ideas, hobbies, routines, notes, or links)", target)
	}
}
