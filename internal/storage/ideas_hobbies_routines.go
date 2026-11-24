package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"personalcrm/internal/domain"
)

// --- Ideas ---

func (s *Store) CreateIdea(ctx context.Context, idea *domain.Idea) error {
	if idea.Status == "" {
		idea.Status = domain.IdeaInbox
	}
	if err := idea.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if idea.ID == "" {
		idea.ID = domain.NewID()
	}
	if idea.CreatedAt.IsZero() {
		idea.CreatedAt = now
	}
	idea.UpdatedAt = now

	var exc sql.NullInt64
	if idea.Excitement != nil {
		exc = sql.NullInt64{Int64: int64(*idea.Excitement), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ideas (id, title, description, status, source, excitement, promoted_project_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		idea.ID, idea.Title, idea.Description, string(idea.Status), idea.Source,
		exc, nullString(idea.PromotedProjectID), formatTime(idea.CreatedAt), formatTime(idea.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert idea: %w", err)
	}
	return nil
}

func (s *Store) GetIdea(ctx context.Context, id string) (domain.Idea, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, description, status, source, excitement, promoted_project_id, created_at, updated_at
		FROM ideas WHERE id = ?
	`, id)
	return scanIdea(row)
}

func (s *Store) ListIdeas(ctx context.Context, statuses ...domain.IdeaStatus) ([]domain.Idea, error) {
	query := `SELECT id, title, description, status, source, excitement, promoted_project_id, created_at, updated_at FROM ideas`
	var args []any
	if len(statuses) > 0 {
		ph := make([]string, len(statuses))
		for i, st := range statuses {
			ph[i] = "?"
			args = append(args, string(st))
		}
		query += " WHERE status IN (" + strings.Join(ph, ",") + ")"
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ideas: %w", err)
	}
	defer rows.Close()

	var out []domain.Idea
	for rows.Next() {
		idea, err := scanIdea(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, idea)
	}
	return out, rows.Err()
}

// ListRefinementIdeas returns ideas eligible for Weekly Refinement triage.
func (s *Store) ListRefinementIdeas(ctx context.Context) ([]domain.Idea, error) {
	return s.ListIdeas(ctx, domain.IdeaInbox, domain.IdeaExploring, domain.IdeaReady)
}

func (s *Store) UpdateIdea(ctx context.Context, idea *domain.Idea) error {
	if err := idea.Validate(); err != nil {
		return err
	}
	idea.UpdatedAt = time.Now().UTC()

	var exc sql.NullInt64
	if idea.Excitement != nil {
		exc = sql.NullInt64{Int64: int64(*idea.Excitement), Valid: true}
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE ideas
		SET title = ?, description = ?, status = ?, source = ?, excitement = ?,
		    promoted_project_id = ?, updated_at = ?
		WHERE id = ?
	`,
		idea.Title, idea.Description, string(idea.Status), idea.Source,
		exc, nullString(idea.PromotedProjectID), formatTime(idea.UpdatedAt), idea.ID,
	)
	if err != nil {
		return fmt.Errorf("update idea %s: %w", idea.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// PromoteIdeaToProject atomically creates an Active Project from an Idea, creates optional
// initial next actions, copies existing links and tags onto the new Project, links any
// specified Hobbies, records an 'inspired_by' link, and marks the Idea as promoted.
func (s *Store) PromoteIdeaToProject(ctx context.Context, in domain.PromoteIdeaInput) (domain.Project, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Project{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT id, title, description, status, source, excitement, promoted_project_id, created_at, updated_at
		FROM ideas WHERE id = ?
	`, in.IdeaID)
	idea, err := scanIdea(row)
	if err != nil {
		return domain.Project{}, err
	}

	now := time.Now().UTC()
	title := strings.TrimSpace(in.ProjectTitle)
	if title == "" {
		title = idea.Title
	}
	desc := in.Description
	if strings.TrimSpace(desc) == "" {
		desc = idea.Description
	}
	prio := in.Priority
	if prio == "" {
		prio = domain.PriorityMedium
	}

	project := domain.Project{
		ID:          domain.NewID(),
		Title:       title,
		Description: desc,
		Goal:        in.Goal,
		Status:      domain.ProjectActive,
		Priority:    prio,
		StartDate:   &now,
		TargetDate:  in.TargetDate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := project.Validate(); err != nil {
		return domain.Project{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO projects (id, title, description, goal, status, priority, start_date, target_date, finished_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)
	`,
		project.ID, project.Title, project.Description, project.Goal, string(project.Status), string(project.Priority),
		formatNullTime(project.StartDate), formatNullTime(project.TargetDate),
		formatTime(project.CreatedAt), formatTime(project.UpdatedAt),
	); err != nil {
		return domain.Project{}, fmt.Errorf("insert promoted project: %w", err)
	}

	// Create initial next actions if provided.
	for i, taskTitle := range in.InitialTasks {
		taskTitle = strings.TrimSpace(taskTitle)
		if taskTitle == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO tasks (id, project_id, title, description, status, priority, position, created_at, updated_at)
			VALUES (?, ?, ?, '', 'open', 'medium', ?, ?, ?)
		`, domain.NewID(), project.ID, taskTitle, i, formatTime(now), formatTime(now)); err != nil {
			return domain.Project{}, fmt.Errorf("insert initial task: %w", err)
		}
	}

	// Carry over tags from Idea -> Project.
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO entity_tags (entity_type, entity_id, tag_id)
		SELECT ?, ?, tag_id FROM entity_tags WHERE entity_type = ? AND entity_id = ?
	`, string(domain.EntityProject), project.ID, string(domain.EntityIdea), idea.ID); err != nil {
		return domain.Project{}, fmt.Errorf("copy idea tags: %w", err)
	}

	// Carry over existing links (notes, resources, hobbies, etc.) from Idea -> Project.
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO entity_links (source_type, source_id, target_type, target_id, relation_type, created_at)
		SELECT ?, ?, target_type, target_id, relation_type, ?
		FROM entity_links
		WHERE source_type = ? AND source_id = ?
	`, string(domain.EntityProject), project.ID, formatTime(now), string(domain.EntityIdea), idea.ID); err != nil {
		return domain.Project{}, fmt.Errorf("copy outgoing idea links: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO entity_links (source_type, source_id, target_type, target_id, relation_type, created_at)
		SELECT source_type, source_id, ?, ?, relation_type, ?
		FROM entity_links
		WHERE target_type = ? AND target_id = ?
	`, string(domain.EntityProject), project.ID, formatTime(now), string(domain.EntityIdea), idea.ID); err != nil {
		return domain.Project{}, fmt.Errorf("copy incoming idea links: %w", err)
	}

	// Link related hobbies if specified.
	for _, hobbyID := range in.HobbyIDs {
		hobbyID = strings.TrimSpace(hobbyID)
		if hobbyID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO entity_links (source_type, source_id, target_type, target_id, relation_type, created_at)
			VALUES (?, ?, ?, ?, 'related', ?)
		`, string(domain.EntityProject), project.ID, string(domain.EntityHobby), hobbyID, formatTime(now)); err != nil {
			return domain.Project{}, fmt.Errorf("link hobby to promoted project: %w", err)
		}
	}

	// Record explicit provenance link Project -> Idea.
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO entity_links (source_type, source_id, target_type, target_id, relation_type, created_at)
		VALUES (?, ?, ?, ?, 'inspired_by', ?)
	`, string(domain.EntityProject), project.ID, string(domain.EntityIdea), idea.ID, formatTime(now)); err != nil {
		return domain.Project{}, fmt.Errorf("link project to idea: %w", err)
	}

	// Mark idea as promoted.
	if _, err := tx.ExecContext(ctx, `
		UPDATE ideas SET status = 'promoted', promoted_project_id = ?, updated_at = ? WHERE id = ?
	`, project.ID, formatTime(now), idea.ID); err != nil {
		return domain.Project{}, fmt.Errorf("mark idea promoted: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.Project{}, err
	}
	return project, nil
}

func (s *Store) DeleteIdea(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM ideas WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete idea %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := cleanupPolymorphicTx(ctx, tx, domain.EntityIdea, id); err != nil {
		return err
	}
	return tx.Commit()
}

func scanIdea(r rowScanner) (domain.Idea, error) {
	var (
		idea                       domain.Idea
		status, createdS, updatedS string
		exc                        sql.NullInt64
		promotedID                 sql.NullString
	)
	err := r.Scan(
		&idea.ID, &idea.Title, &idea.Description, &status, &idea.Source,
		&exc, &promotedID, &createdS, &updatedS,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Idea{}, ErrNotFound
		}
		return domain.Idea{}, fmt.Errorf("scan idea: %w", err)
	}
	idea.Status = domain.IdeaStatus(status)
	if exc.Valid {
		v := int(exc.Int64)
		idea.Excitement = &v
	}
	idea.PromotedProjectID = promotedID.String
	if idea.CreatedAt, err = parseTime(createdS); err != nil {
		return domain.Idea{}, err
	}
	if idea.UpdatedAt, err = parseTime(updatedS); err != nil {
		return domain.Idea{}, err
	}
	return idea, nil
}

// --- Hobbies ---

func (s *Store) CreateHobby(ctx context.Context, h *domain.Hobby) error {
	if h.Status == "" {
		h.Status = domain.HobbyActive
	}
	if err := h.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if h.ID == "" {
		h.ID = domain.NewID()
	}
	if h.CreatedAt.IsZero() {
		h.CreatedAt = now
	}
	h.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO hobbies (id, name, description, current_focus, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, h.ID, h.Name, h.Description, h.CurrentFocus, string(h.Status), formatTime(h.CreatedAt), formatTime(h.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert hobby: %w", err)
	}
	return nil
}

func (s *Store) GetHobby(ctx context.Context, id string) (domain.Hobby, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, current_focus, status, created_at, updated_at
		FROM hobbies WHERE id = ?
	`, id)
	return scanHobby(row)
}

func (s *Store) ListHobbies(ctx context.Context, statuses ...domain.HobbyStatus) ([]domain.Hobby, error) {
	query := `SELECT id, name, description, current_focus, status, created_at, updated_at FROM hobbies`
	var args []any
	if len(statuses) > 0 {
		ph := make([]string, len(statuses))
		for i, st := range statuses {
			ph[i] = "?"
			args = append(args, string(st))
		}
		query += " WHERE status IN (" + strings.Join(ph, ",") + ")"
	}
	query += ` ORDER BY CASE status WHEN 'active' THEN 0 WHEN 'on_hold' THEN 1 ELSE 2 END, name ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list hobbies: %w", err)
	}
	defer rows.Close()

	var out []domain.Hobby
	for rows.Next() {
		h, err := scanHobby(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) UpdateHobby(ctx context.Context, h *domain.Hobby) error {
	if err := h.Validate(); err != nil {
		return err
	}
	h.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE hobbies SET name = ?, description = ?, current_focus = ?, status = ?, updated_at = ?
		WHERE id = ?
	`, h.Name, h.Description, h.CurrentFocus, string(h.Status), formatTime(h.UpdatedAt), h.ID)
	if err != nil {
		return fmt.Errorf("update hobby %s: %w", h.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteHobby(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM hobbies WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete hobby %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := cleanupPolymorphicTx(ctx, tx, domain.EntityHobby, id); err != nil {
		return err
	}
	return tx.Commit()
}

func scanHobby(r rowScanner) (domain.Hobby, error) {
	var (
		h                          domain.Hobby
		status, createdS, updatedS string
	)
	err := r.Scan(&h.ID, &h.Name, &h.Description, &h.CurrentFocus, &status, &createdS, &updatedS)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Hobby{}, ErrNotFound
		}
		return domain.Hobby{}, fmt.Errorf("scan hobby: %w", err)
	}
	h.Status = domain.HobbyStatus(status)
	if h.CreatedAt, err = parseTime(createdS); err != nil {
		return domain.Hobby{}, err
	}
	if h.UpdatedAt, err = parseTime(updatedS); err != nil {
		return domain.Hobby{}, err
	}
	return h, nil
}

// --- Routines ---

func (s *Store) CreateRoutine(ctx context.Context, r *domain.Routine) error {
	if err := r.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if r.ID == "" {
		r.ID = domain.NewID()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now

	cfgJSON, err := json.Marshal(r.ScheduleConfig)
	if err != nil {
		return fmt.Errorf("marshal schedule_config: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO routines (
			id, title, description, schedule_type, schedule_config, active,
			reminder_enabled, reminder_time, completed_cycle_key, last_completed_at,
			last_reminded_cycle, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		r.ID, r.Title, r.Description, string(r.ScheduleType), string(cfgJSON),
		boolToInt(r.Active), boolToInt(r.ReminderEnabled), r.ReminderTime,
		r.CompletedCycleKey, formatNullTime(r.LastCompletedAt), r.LastRemindedCycle,
		formatTime(r.CreatedAt), formatTime(r.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert routine: %w", err)
	}
	return nil
}

func (s *Store) GetRoutine(ctx context.Context, id string) (domain.Routine, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, description, schedule_type, schedule_config, active,
		       reminder_enabled, reminder_time, completed_cycle_key, last_completed_at,
		       last_reminded_cycle, created_at, updated_at
		FROM routines WHERE id = ?
	`, id)
	return scanRoutine(row)
}

func (s *Store) ListRoutines(ctx context.Context, activeOnly bool) ([]domain.Routine, error) {
	query := `
		SELECT id, title, description, schedule_type, schedule_config, active,
		       reminder_enabled, reminder_time, completed_cycle_key, last_completed_at,
		       last_reminded_cycle, created_at, updated_at
		FROM routines`
	if activeOnly {
		query += ` WHERE active = 1`
	}
	query += ` ORDER BY active DESC, title ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list routines: %w", err)
	}
	defer rows.Close()

	var out []domain.Routine
	for rows.Next() {
		r, err := scanRoutine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) UpdateRoutine(ctx context.Context, r *domain.Routine) error {
	if err := r.Validate(); err != nil {
		return err
	}
	r.UpdatedAt = time.Now().UTC()
	cfgJSON, err := json.Marshal(r.ScheduleConfig)
	if err != nil {
		return fmt.Errorf("marshal schedule_config: %w", err)
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE routines
		SET title = ?, description = ?, schedule_type = ?, schedule_config = ?,
		    active = ?, reminder_enabled = ?, reminder_time = ?,
		    completed_cycle_key = ?, last_completed_at = ?, last_reminded_cycle = ?,
		    updated_at = ?
		WHERE id = ?
	`,
		r.Title, r.Description, string(r.ScheduleType), string(cfgJSON),
		boolToInt(r.Active), boolToInt(r.ReminderEnabled), r.ReminderTime,
		r.CompletedCycleKey, formatNullTime(r.LastCompletedAt), r.LastRemindedCycle,
		formatTime(r.UpdatedAt), r.ID,
	)
	if err != nil {
		return fmt.Errorf("update routine %s: %w", r.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetRoutineCompleted(ctx context.Context, id string, cycleKey string, completedAt time.Time) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE routines
		SET completed_cycle_key = ?, last_completed_at = ?, updated_at = ?
		WHERE id = ?
	`, cycleKey, formatTime(completedAt), formatTime(time.Now().UTC()), id)
	if err != nil {
		return fmt.Errorf("complete routine %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ClearRoutineCompleted(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE routines
		SET completed_cycle_key = '', updated_at = ?
		WHERE id = ?
	`, formatTime(time.Now().UTC()), id)
	if err != nil {
		return fmt.Errorf("clear routine completion %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MarkRoutineReminded(ctx context.Context, id string, cycleKey string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE routines SET last_reminded_cycle = ?, updated_at = ? WHERE id = ?
	`, cycleKey, formatTime(time.Now().UTC()), id)
	if err != nil {
		return fmt.Errorf("mark routine reminded %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteRoutine(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM routines WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete routine %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := cleanupPolymorphicTx(ctx, tx, domain.EntityRoutine, id); err != nil {
		return err
	}
	return tx.Commit()
}

func scanRoutine(row rowScanner) (domain.Routine, error) {
	var (
		r                                      domain.Routine
		schedType, cfgJSON, createdS, updatedS string
		active, remEnabled                     int
		lastCompS                              sql.NullString
	)
	err := row.Scan(
		&r.ID, &r.Title, &r.Description, &schedType, &cfgJSON, &active,
		&remEnabled, &r.ReminderTime, &r.CompletedCycleKey, &lastCompS,
		&r.LastRemindedCycle, &createdS, &updatedS,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Routine{}, ErrNotFound
		}
		return domain.Routine{}, fmt.Errorf("scan routine: %w", err)
	}
	r.ScheduleType = domain.ScheduleType(schedType)
	r.Active = active != 0
	r.ReminderEnabled = remEnabled != 0
	if strings.TrimSpace(cfgJSON) != "" {
		if err := json.Unmarshal([]byte(cfgJSON), &r.ScheduleConfig); err != nil {
			return domain.Routine{}, fmt.Errorf("unmarshal schedule_config: %w", err)
		}
	}
	if r.LastCompletedAt, err = parseNullTime(lastCompS); err != nil {
		return domain.Routine{}, err
	}
	if r.CreatedAt, err = parseTime(createdS); err != nil {
		return domain.Routine{}, err
	}
	if r.UpdatedAt, err = parseTime(updatedS); err != nil {
		return domain.Routine{}, err
	}
	return r, nil
}
