package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"personalcrm/internal/domain"
)

// cleanupPolymorphicTx removes orphaned links, tags, and progress logs when an entity is deleted.
func cleanupPolymorphicTx(ctx context.Context, tx *sql.Tx, entityType domain.EntityType, entityID string) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM entity_links
		WHERE (source_type = ? AND source_id = ?) OR (target_type = ? AND target_id = ?)
	`, string(entityType), entityID, string(entityType), entityID); err != nil {
		return fmt.Errorf("cleanup entity_links: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM entity_tags WHERE entity_type = ? AND entity_id = ?
	`, string(entityType), entityID); err != nil {
		return fmt.Errorf("cleanup entity_tags: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM progress_logs WHERE entity_type = ? AND entity_id = ?
	`, string(entityType), entityID); err != nil {
		return fmt.Errorf("cleanup progress_logs: %w", err)
	}
	return nil
}

// --- Polymorphic Entity Links ---

func (s *Store) AddLink(ctx context.Context, link domain.EntityLink) error {
	if strings.TrimSpace(link.RelationType) == "" {
		link.RelationType = "related"
	}
	if err := link.Validate(); err != nil {
		return err
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO entity_links (source_type, source_id, target_type, target_id, relation_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		string(link.SourceType), link.SourceID,
		string(link.TargetType), link.TargetID,
		link.RelationType, formatTime(link.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("add entity link: %w", err)
	}
	return nil
}

func (s *Store) RemoveLink(ctx context.Context, link domain.EntityLink) error {
	if strings.TrimSpace(link.RelationType) == "" {
		link.RelationType = "related"
	}
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM entity_links
		WHERE source_type = ? AND source_id = ? AND target_type = ? AND target_id = ? AND relation_type = ?
	`, string(link.SourceType), link.SourceID, string(link.TargetType), link.TargetID, link.RelationType)
	if err != nil {
		return fmt.Errorf("remove entity link: %w", err)
	}
	return nil
}

// ListLinksFor returns all links involving the given entity (either as source or target).
func (s *Store) ListLinksFor(ctx context.Context, entityType domain.EntityType, entityID string) ([]domain.EntityLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_type, source_id, target_type, target_id, relation_type, created_at
		FROM entity_links
		WHERE (source_type = ? AND source_id = ?) OR (target_type = ? AND target_id = ?)
		ORDER BY created_at DESC
	`, string(entityType), entityID, string(entityType), entityID)
	if err != nil {
		return nil, fmt.Errorf("list links for %s:%s: %w", entityType, entityID, err)
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

// --- Polymorphic Tags ---

func (s *Store) EnsureTag(ctx context.Context, name string) (domain.Tag, error) {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return domain.Tag{}, errors.New("tag name is required")
	}

	var tag domain.Tag
	err := s.db.QueryRowContext(ctx, `SELECT id, name FROM tags WHERE name = ? COLLATE NOCASE`, clean).
		Scan(&tag.ID, &tag.Name)
	if err == nil {
		return tag, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.Tag{}, fmt.Errorf("lookup tag %q: %w", clean, err)
	}

	tag = domain.Tag{ID: domain.NewID(), Name: clean}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO tags (id, name) VALUES (?, ?)`, tag.ID, tag.Name); err != nil {
		return domain.Tag{}, fmt.Errorf("insert tag %q: %w", clean, err)
	}
	return tag, nil
}

func (s *Store) TagEntity(ctx context.Context, entityType domain.EntityType, entityID string, tagName string) (domain.Tag, error) {
	tag, err := s.EnsureTag(ctx, tagName)
	if err != nil {
		return domain.Tag{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO entity_tags (entity_type, entity_id, tag_id) VALUES (?, ?, ?)
	`, string(entityType), entityID, tag.ID); err != nil {
		return domain.Tag{}, fmt.Errorf("tag entity: %w", err)
	}
	return tag, nil
}

func (s *Store) UntagEntity(ctx context.Context, entityType domain.EntityType, entityID string, tagName string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM entity_tags
		WHERE entity_type = ? AND entity_id = ?
		  AND tag_id IN (SELECT id FROM tags WHERE name = ? COLLATE NOCASE)
	`, string(entityType), entityID, strings.TrimSpace(tagName))
	if err != nil {
		return fmt.Errorf("untag entity: %w", err)
	}
	return nil
}

func (s *Store) ListTagsFor(ctx context.Context, entityType domain.EntityType, entityID string) ([]domain.Tag, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.name
		FROM tags t
		INNER JOIN entity_tags et ON et.tag_id = t.id
		WHERE et.entity_type = ? AND et.entity_id = ?
		ORDER BY t.name ASC
	`, string(entityType), entityID)
	if err != nil {
		return nil, fmt.Errorf("list tags for %s:%s: %w", entityType, entityID, err)
	}
	defer rows.Close()

	var out []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) ListAllTags(ctx context.Context) ([]domain.Tag, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM tags ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list all tags: %w", err)
	}
	defer rows.Close()

	var out []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// --- Polymorphic Progress Logs ---

func (s *Store) AddProgressLog(ctx context.Context, entry *domain.ProgressLog) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if entry.ID == "" {
		entry.ID = domain.NewID()
	}
	if entry.LoggedAt.IsZero() {
		entry.LoggedAt = now
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO progress_logs (id, entity_type, entity_id, logged_at, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, entry.ID, string(entry.EntityType), entry.EntityID, formatTime(entry.LoggedAt), entry.Content, formatTime(entry.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert progress log: %w", err)
	}
	return nil
}

func (s *Store) ListProgressLogs(ctx context.Context, entityType domain.EntityType, entityID string) ([]domain.ProgressLog, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, logged_at, content, created_at
		FROM progress_logs
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY logged_at DESC, created_at DESC
	`, string(entityType), entityID)
	if err != nil {
		return nil, fmt.Errorf("list progress logs: %w", err)
	}
	defer rows.Close()

	var out []domain.ProgressLog
	for rows.Next() {
		var (
			p                        domain.ProgressLog
			et, loggedS, createdS    string
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

func (s *Store) DeleteProgressLog(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM progress_logs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete progress log %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Refinement Runs & App State ---

const (
	StateRefinementLastCompleted = "refinement.last_completed_at"
	StateRefinementSnoozedUntil  = "refinement.snoozed_until"
	StateRefinementNotifiedCycle = "refinement.notified_cycle"
)

func (s *Store) StartRefinementRun(ctx context.Context, cycleKey string) (domain.RefinementRun, error) {
	now := time.Now().UTC()
	run := domain.RefinementRun{
		ID:        domain.NewID(),
		CycleKey:  cycleKey,
		StartedAt: now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO refinement_runs (id, cycle_key, started_at, projects_reviewed, ideas_reviewed, routines_reviewed, notes)
		VALUES (?, ?, ?, 0, 0, 0, '')
	`, run.ID, run.CycleKey, formatTime(run.StartedAt))
	if err != nil {
		return domain.RefinementRun{}, fmt.Errorf("start refinement run: %w", err)
	}
	return run, nil
}

func (s *Store) CompleteRefinementRun(ctx context.Context, run *domain.RefinementRun) error {
	now := time.Now().UTC()
	if run.CompletedAt == nil {
		run.CompletedAt = &now
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		UPDATE refinement_runs
		SET completed_at = ?, projects_reviewed = ?, ideas_reviewed = ?, routines_reviewed = ?, notes = ?
		WHERE id = ?
	`, formatTime(*run.CompletedAt), run.ProjectsReviewed, run.IdeasReviewed, run.RoutinesReviewed, run.Notes, run.ID)
	if err != nil {
		return fmt.Errorf("complete refinement run: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}

	if err := setAppStateTx(ctx, tx, StateRefinementLastCompleted, formatTime(*run.CompletedAt)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM app_state WHERE key IN (?, ?)`,
		StateRefinementSnoozedUntil, StateRefinementNotifiedCycle); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) ListRefinementRuns(ctx context.Context, limit int) ([]domain.RefinementRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, cycle_key, started_at, completed_at, projects_reviewed, ideas_reviewed, routines_reviewed, notes
		FROM refinement_runs
		ORDER BY started_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list refinement runs: %w", err)
	}
	defer rows.Close()

	var out []domain.RefinementRun
	for rows.Next() {
		var (
			r        domain.RefinementRun
			startedS string
			compS    sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.CycleKey, &startedS, &compS, &r.ProjectsReviewed, &r.IdeasReviewed, &r.RoutinesReviewed, &r.Notes); err != nil {
			return nil, err
		}
		if r.StartedAt, err = parseTime(startedS); err != nil {
			return nil, err
		}
		if r.CompletedAt, err = parseNullTime(compS); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetAppState(ctx context.Context, key string) (string, bool, error) {
	var val string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_state WHERE key = ?`, key).Scan(&val)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("get app_state %q: %w", key, err)
	}
	return val, true, nil
}

func (s *Store) SetAppState(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_state (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, formatTime(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("set app_state %q: %w", key, err)
	}
	return nil
}

func (s *Store) DeleteAppState(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM app_state WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete app_state %q: %w", key, err)
	}
	return nil
}

func setAppStateTx(ctx context.Context, tx *sql.Tx, key, value string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO app_state (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, formatTime(time.Now().UTC()))
	return err
}

// GetRefinementState loads persisted scheduling timestamps used by domain.EvaluateRefinement.
func (s *Store) GetRefinementState(ctx context.Context) (domain.RefinementState, error) {
	var st domain.RefinementState

	if v, ok, err := s.GetAppState(ctx, StateRefinementLastCompleted); err != nil {
		return st, err
	} else if ok && v != "" {
		t, err := parseTime(v)
		if err != nil {
			return st, err
		}
		st.LastCompletedAt = &t
	}

	if v, ok, err := s.GetAppState(ctx, StateRefinementSnoozedUntil); err != nil {
		return st, err
	} else if ok && v != "" {
		t, err := parseTime(v)
		if err != nil {
			return st, err
		}
		st.SnoozedUntil = &t
	}

	if v, ok, err := s.GetAppState(ctx, StateRefinementNotifiedCycle); err != nil {
		return st, err
	} else if ok {
		st.NotifiedCycle = v
	}

	return st, nil
}

// SnoozeRefinement defers refinement notifications until until and clears NotifiedCycle
// so a fresh notification fires once the snooze window expires.
func (s *Store) SnoozeRefinement(ctx context.Context, until time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := setAppStateTx(ctx, tx, StateRefinementSnoozedUntil, formatTime(until)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM app_state WHERE key = ?`, StateRefinementNotifiedCycle); err != nil {
		return err
	}
	return tx.Commit()
}
