package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"personalcrm/internal/domain"
)

// ExportSnapshot is a portable JSON snapshot of all core PersonalCRM data.
type ExportSnapshot struct {
	ExportedAt     time.Time              `json:"exported_at"`
	SchemaVersion  int                    `json:"schema_version"`
	Projects       []domain.Project       `json:"projects"`
	Tasks          []domain.Task          `json:"tasks"`
	Ideas          []domain.Idea          `json:"ideas"`
	Hobbies        []domain.Hobby         `json:"hobbies"`
	Routines       []domain.Routine       `json:"routines"`
	Notes          []domain.Note          `json:"notes"`
	Resources      []domain.Resource      `json:"resources"`
	Tags           []domain.Tag           `json:"tags"`
	EntityTags     []ExportEntityTag      `json:"entity_tags"`
	EntityLinks    []domain.EntityLink    `json:"entity_links"`
	ProgressLogs   []domain.ProgressLog   `json:"progress_logs"`
	RefinementRuns []domain.RefinementRun `json:"refinement_runs"`
}

// ExportEntityTag represents a polymorphic tag assignment in exports.
type ExportEntityTag struct {
	EntityType domain.EntityType `json:"entity_type"`
	EntityID   string            `json:"entity_id"`
	TagID      string            `json:"tag_id"`
}

// Backup creates a consistent standalone SQLite copy at destPath using VACUUM INTO,
// safe to invoke while WAL readers/writers are active.
func (s *Store) Backup(ctx context.Context, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	// VACUUM INTO refuses to overwrite an existing file.
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing backup target: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, destPath); err != nil {
		return fmt.Errorf("vacuum into %s: %w", destPath, err)
	}
	return nil
}

// ExportSnapshot collects all core entities and polymorphic relations for export.
func (s *Store) ExportSnapshot(ctx context.Context) (ExportSnapshot, error) {
	snap := ExportSnapshot{
		ExportedAt:    time.Now().UTC(),
		SchemaVersion: 1,
	}

	var err error
	if snap.Projects, err = s.ListProjects(ctx, ProjectFilter{}); err != nil {
		return snap, err
	}
	if snap.Tasks, err = s.listAllTasks(ctx); err != nil {
		return snap, err
	}
	if snap.Ideas, err = s.ListIdeas(ctx); err != nil {
		return snap, err
	}
	if snap.Hobbies, err = s.ListHobbies(ctx); err != nil {
		return snap, err
	}
	if snap.Routines, err = s.ListRoutines(ctx, false); err != nil {
		return snap, err
	}
	if snap.Notes, err = s.ListNotes(ctx, ""); err != nil {
		return snap, err
	}
	if snap.Resources, err = s.ListResources(ctx, ""); err != nil {
		return snap, err
	}
	if snap.Tags, err = s.ListAllTags(ctx); err != nil {
		return snap, err
	}
	if snap.EntityTags, err = s.listAllEntityTags(ctx); err != nil {
		return snap, err
	}
	if snap.EntityLinks, err = s.listAllLinks(ctx); err != nil {
		return snap, err
	}
	if snap.ProgressLogs, err = s.listAllProgressLogs(ctx); err != nil {
		return snap, err
	}
	if snap.RefinementRuns, err = s.ListRefinementRuns(ctx, 1000); err != nil {
		return snap, err
	}
	return snap, nil
}

// ExportJSON encodes the full snapshot as pretty-printed JSON.
func (s *Store) ExportJSON(ctx context.Context) ([]byte, error) {
	snap, err := s.ExportSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(snap, "", "  ")
}

func (s *Store) listAllTasks(ctx context.Context) ([]domain.Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, title, description, status, priority, due_at, reminder_enabled, reminder_sent_at, position, completed_at, created_at, updated_at
		FROM tasks ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (s *Store) listAllEntityTags(ctx context.Context) ([]ExportEntityTag, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT entity_type, entity_id, tag_id FROM entity_tags ORDER BY entity_type, entity_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExportEntityTag
	for rows.Next() {
		var et ExportEntityTag
		var typ string
		if err := rows.Scan(&typ, &et.EntityID, &et.TagID); err != nil {
			return nil, err
		}
		et.EntityType = domain.EntityType(typ)
		out = append(out, et)
	}
	return out, rows.Err()
}

func (s *Store) listAllLinks(ctx context.Context) ([]domain.EntityLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_type, source_id, target_type, target_id, relation_type, created_at
		FROM entity_links ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.EntityLink
	for rows.Next() {
		var (
			l                        domain.EntityLink
			srcType, tgtType, crTime string
		)
		if err := rows.Scan(&srcType, &l.SourceID, &tgtType, &l.TargetID, &l.RelationType, &crTime); err != nil {
			return nil, err
		}
		l.SourceType = domain.EntityType(srcType)
		l.TargetType = domain.EntityType(tgtType)
		if l.CreatedAt, err = parseTime(crTime); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) listAllProgressLogs(ctx context.Context) ([]domain.ProgressLog, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, logged_at, content, created_at
		FROM progress_logs ORDER BY logged_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ProgressLog
	for rows.Next() {
		var (
			p                     domain.ProgressLog
			et, loggedS, createdS string
		)
		if err := rows.Scan(&p.ID, &et, &p.EntityID, &loggedS, &p.Content, &createdS); err != nil {
			return nil, err
		}
		p.EntityType = domain.EntityType(et)
		if p.LoggedAt, err = parseTime(loggedS); err != nil {
			return nil, err
		}
		if p.CreatedAt, err = parseTime(createdS); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
