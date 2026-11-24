package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"personalcrm/internal/domain"
)

// ExportMarkdown writes a human-readable, Obsidian-compatible folder of Markdown
// files with YAML frontmatter for all core entities, tasks, logs, notes, and resources.
func (s *Store) ExportMarkdown(ctx context.Context, outDir string) error {
	snap, err := s.ExportSnapshot(ctx)
	if err != nil {
		return err
	}

	subDirs := []string{
		outDir,
		filepath.Join(outDir, "projects"),
		filepath.Join(outDir, "ideas"),
		filepath.Join(outDir, "hobbies"),
		filepath.Join(outDir, "notes"),
	}
	for _, d := range subDirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create export dir %s: %w", d, err)
		}
	}

	// Build fast lookup maps for tags, tasks, progress logs, and links.
	tagByID := make(map[string]string, len(snap.Tags))
	for _, t := range snap.Tags {
		tagByID[t.ID] = t.Name
	}
	tagsByEntity := map[string][]string{}
	for _, et := range snap.EntityTags {
		key := entityKey(et.EntityType, et.EntityID)
		if name, ok := tagByID[et.TagID]; ok {
			tagsByEntity[key] = append(tagsByEntity[key], name)
		}
	}

	tasksByProject := map[string][]domain.Task{}
	for _, t := range snap.Tasks {
		tasksByProject[t.ProjectID] = append(tasksByProject[t.ProjectID], t)
	}

	logsByEntity := map[string][]domain.ProgressLog{}
	for _, l := range snap.ProgressLogs {
		key := entityKey(l.EntityType, l.EntityID)
		logsByEntity[key] = append(logsByEntity[key], l)
	}

	linksByEntity := map[string][]domain.EntityLink{}
	for _, lk := range snap.EntityLinks {
		sk := entityKey(lk.SourceType, lk.SourceID)
		tk := entityKey(lk.TargetType, lk.TargetID)
		linksByEntity[sk] = append(linksByEntity[sk], lk)
		linksByEntity[tk] = append(linksByEntity[tk], lk)
	}

	// Export Projects.
	for _, p := range snap.Projects {
		key := entityKey(domain.EntityProject, p.ID)
		var b strings.Builder
		b.WriteString("---\n")
		writeYAMLField(&b, "id", p.ID)
		writeYAMLField(&b, "type", "project")
		writeYAMLField(&b, "title", p.Title)
		writeYAMLField(&b, "status", string(p.Status))
		writeYAMLField(&b, "priority", string(p.Priority))
		writeYAMLList(&b, "tags", tagsByEntity[key])
		writeYAMLField(&b, "created_at", formatTime(p.CreatedAt))
		b.WriteString("---\n\n")
		fmt.Fprintf(&b, "# %s\n\n", p.Title)
		if p.Goal != "" {
			fmt.Fprintf(&b, "**Goal:** %s\n\n", p.Goal)
		}
		if p.Description != "" {
			b.WriteString(p.Description + "\n\n")
		}
		if tasks := tasksByProject[p.ID]; len(tasks) > 0 {
			b.WriteString("## Tasks\n\n")
			for _, t := range tasks {
				check := " "
				if t.Status == domain.TaskDone {
					check = "x"
				}
				fmt.Fprintf(&b, "- [%s] %s (%s)\n", check, t.Title, t.Status)
			}
			b.WriteString("\n")
		}
		writeProgressLogsMD(&b, logsByEntity[key])
		writeLinksMD(&b, linksByEntity[key])

		file := filepath.Join(outDir, "projects", mdFileName(p.Title, p.ID))
		if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}

	// Export Ideas.
	for _, idea := range snap.Ideas {
		key := entityKey(domain.EntityIdea, idea.ID)
		var b strings.Builder
		b.WriteString("---\n")
		writeYAMLField(&b, "id", idea.ID)
		writeYAMLField(&b, "type", "idea")
		writeYAMLField(&b, "title", idea.Title)
		writeYAMLField(&b, "status", string(idea.Status))
		if idea.Source != "" {
			writeYAMLField(&b, "source", idea.Source)
		}
		if idea.PromotedProjectID != "" {
			writeYAMLField(&b, "promoted_project_id", idea.PromotedProjectID)
		}
		writeYAMLList(&b, "tags", tagsByEntity[key])
		writeYAMLField(&b, "created_at", formatTime(idea.CreatedAt))
		b.WriteString("---\n\n")
		fmt.Fprintf(&b, "# %s\n\n", idea.Title)
		if idea.Description != "" {
			b.WriteString(idea.Description + "\n\n")
		}
		writeLinksMD(&b, linksByEntity[key])

		file := filepath.Join(outDir, "ideas", mdFileName(idea.Title, idea.ID))
		if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}

	// Export Hobbies.
	for _, h := range snap.Hobbies {
		key := entityKey(domain.EntityHobby, h.ID)
		var b strings.Builder
		b.WriteString("---\n")
		writeYAMLField(&b, "id", h.ID)
		writeYAMLField(&b, "type", "hobby")
		writeYAMLField(&b, "name", h.Name)
		writeYAMLField(&b, "status", string(h.Status))
		writeYAMLList(&b, "tags", tagsByEntity[key])
		writeYAMLField(&b, "created_at", formatTime(h.CreatedAt))
		b.WriteString("---\n\n")
		fmt.Fprintf(&b, "# %s\n\n", h.Name)
		if h.CurrentFocus != "" {
			fmt.Fprintf(&b, "**Current Focus:** %s\n\n", h.CurrentFocus)
		}
		if h.Description != "" {
			b.WriteString(h.Description + "\n\n")
		}
		writeProgressLogsMD(&b, logsByEntity[key])
		writeLinksMD(&b, linksByEntity[key])

		file := filepath.Join(outDir, "hobbies", mdFileName(h.Name, h.ID))
		if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}

	// Export Notes.
	for _, n := range snap.Notes {
		key := entityKey(domain.EntityNote, n.ID)
		var b strings.Builder
		b.WriteString("---\n")
		writeYAMLField(&b, "id", n.ID)
		writeYAMLField(&b, "type", "note")
		writeYAMLField(&b, "title", n.Title)
		if n.Source != "" {
			writeYAMLField(&b, "source", n.Source)
		}
		writeYAMLList(&b, "tags", tagsByEntity[key])
		writeYAMLField(&b, "updated_at", formatTime(n.UpdatedAt))
		b.WriteString("---\n\n")
		fmt.Fprintf(&b, "# %s\n\n", n.Title)
		if n.Content != "" {
			b.WriteString(n.Content + "\n\n")
		}
		writeLinksMD(&b, linksByEntity[key])

		file := filepath.Join(outDir, "notes", mdFileName(n.Title, n.ID))
		if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}

	// Export Resources summary.
	{
		var b strings.Builder
		b.WriteString("# Resources & Links\n\n")
		if len(snap.Resources) == 0 {
			b.WriteString("_No resources recorded._\n")
		} else {
			b.WriteString("| Title | Type | Status | URL / Path | Notes |\n")
			b.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
			for _, r := range snap.Resources {
				fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
					escapeMDCell(r.Title), r.ResourceType, r.Status, escapeMDCell(r.URLOrPath), escapeMDCell(r.Notes))
			}
		}
		if err := os.WriteFile(filepath.Join(outDir, "resources.md"), []byte(b.String()), 0o644); err != nil {
			return err
		}
	}

	// Export Routines summary.
	{
		var b strings.Builder
		b.WriteString("# Routines\n\n")
		if len(snap.Routines) == 0 {
			b.WriteString("_No routines configured._\n")
		} else {
			for _, r := range snap.Routines {
				state := "active"
				if !r.Active {
					state = "paused"
				}
				fmt.Fprintf(&b, "- **%s** — `%s` (%s)\n", r.Title, r.ScheduleType, state)
			}
		}
		if err := os.WriteFile(filepath.Join(outDir, "routines.md"), []byte(b.String()), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func entityKey(t domain.EntityType, id string) string {
	return string(t) + ":" + id
}

func writeYAMLField(b *strings.Builder, key, val string) {
	escaped := strings.ReplaceAll(val, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	fmt.Fprintf(b, "%s: \"%s\"\n", key, escaped)
}

func writeYAMLList(b *strings.Builder, key string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", key)
	for _, it := range items {
		escaped := strings.ReplaceAll(it, `"`, `\"`)
		fmt.Fprintf(b, "  - \"%s\"\n", escaped)
	}
}

func writeProgressLogsMD(b *strings.Builder, logs []domain.ProgressLog) {
	if len(logs) == 0 {
		return
	}
	b.WriteString("## Progress Log\n\n")
	for _, l := range logs {
		fmt.Fprintf(b, "- **%s:** %s\n", l.LoggedAt.Format("2006-01-02"), l.Content)
	}
	b.WriteString("\n")
}

func writeLinksMD(b *strings.Builder, links []domain.EntityLink) {
	if len(links) == 0 {
		return
	}
	b.WriteString("## Linked Entities\n\n")
	for _, lk := range links {
		fmt.Fprintf(b, "- `%s:%s` —(%s)→ `%s:%s`\n", lk.SourceType, lk.SourceID, lk.RelationType, lk.TargetType, lk.TargetID)
	}
	b.WriteString("\n")
}

func mdFileName(title, id string) string {
	var slug strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			slug.WriteRune(r)
			lastDash = false
		} else if !lastDash && slug.Len() > 0 {
			slug.WriteByte('-')
			lastDash = true
		}
		if slug.Len() >= 40 {
			break
		}
	}
	s := strings.Trim(slug.String(), "-")
	if s == "" {
		s = "item"
	}
	shortID := id
	if len(shortID) > 8 {
		shortID = shortID[len(shortID)-8:]
	}
	return fmt.Sprintf("%s-%s.md", s, strings.ToLower(shortID))
}

func escapeMDCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "|", `\|`)
}
