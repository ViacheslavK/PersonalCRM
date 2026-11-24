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

// ProjectFilter filters listed projects.
type ProjectFilter struct {
	Statuses []domain.ProjectStatus
	Priority domain.Priority
	Query    string
}

func (s *Store) CreateProject(ctx context.Context, p *domain.Project) error {
	if p.Status == "" {
		p.Status = domain.ProjectActive
	}
	if p.Priority == "" {
		p.Priority = domain.PriorityMedium
	}
	if err := p.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if p.ID == "" {
		p.ID = domain.NewID()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	if p.Status == domain.ProjectFinished && p.FinishedAt == nil {
		p.FinishedAt = &now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects (id, title, description, goal, status, priority, start_date, target_date, finished_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		p.ID, p.Title, p.Description, p.Goal, string(p.Status), string(p.Priority),
		formatNullTime(p.StartDate), formatNullTime(p.TargetDate), formatNullTime(p.FinishedAt),
		formatTime(p.CreatedAt), formatTime(p.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

func (s *Store) GetProject(ctx context.Context, id string) (domain.Project, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, description, goal, status, priority, start_date, target_date, finished_at, created_at, updated_at
		FROM projects WHERE id = ?
	`, id)
	return scanProject(row)
}

func (s *Store) ListProjects(ctx context.Context, filter ProjectFilter) ([]domain.Project, error) {
	var where []string
	var args []any

	if len(filter.Statuses) > 0 {
		placeholders := make([]string, len(filter.Statuses))
		for i, st := range filter.Statuses {
			placeholders[i] = "?"
			args = append(args, string(st))
		}
		where = append(where, "status IN ("+strings.Join(placeholders, ",")+")")
	}
	if filter.Priority != "" {
		where = append(where, "priority = ?")
		args = append(args, string(filter.Priority))
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		like := "%" + q + "%"
		where = append(where, "(title LIKE ? OR description LIKE ? OR goal LIKE ?)")
		args = append(args, like, like, like)
	}

	query := `SELECT id, title, description, goal, status, priority, start_date, target_date, finished_at, created_at, updated_at FROM projects`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += ` ORDER BY CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, updated_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var out []domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListRefinementProjects returns projects eligible for Weekly Refinement
// (status NOT IN ('finished', 'rejected', 'on_hold')).
func (s *Store) ListRefinementProjects(ctx context.Context) ([]domain.Project, error) {
	return s.ListProjects(ctx, ProjectFilter{
		Statuses: []domain.ProjectStatus{domain.ProjectActive, domain.ProjectWaiting},
	})
}

func (s *Store) UpdateProject(ctx context.Context, p *domain.Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	p.UpdatedAt = now
	if p.Status == domain.ProjectFinished && p.FinishedAt == nil {
		p.FinishedAt = &now
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET title = ?, description = ?, goal = ?, status = ?, priority = ?,
		    start_date = ?, target_date = ?, finished_at = ?, updated_at = ?
		WHERE id = ?
	`,
		p.Title, p.Description, p.Goal, string(p.Status), string(p.Priority),
		formatNullTime(p.StartDate), formatNullTime(p.TargetDate), formatNullTime(p.FinishedAt),
		formatTime(p.UpdatedAt), p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project %s: %w", p.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := cleanupPolymorphicTx(ctx, tx, domain.EntityProject, id); err != nil {
		return err
	}
	return tx.Commit()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(r rowScanner) (domain.Project, error) {
	var (
		p                               domain.Project
		status, priority                string
		startS, targetS, finishedS      sql.NullString
		createdS, updatedS              string
	)
	err := r.Scan(
		&p.ID, &p.Title, &p.Description, &p.Goal, &status, &priority,
		&startS, &targetS, &finishedS, &createdS, &updatedS,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Project{}, ErrNotFound
		}
		return domain.Project{}, fmt.Errorf("scan project: %w", err)
	}
	p.Status = domain.ProjectStatus(status)
	p.Priority = domain.Priority(priority)
	if p.StartDate, err = parseNullTime(startS); err != nil {
		return domain.Project{}, err
	}
	if p.TargetDate, err = parseNullTime(targetS); err != nil {
		return domain.Project{}, err
	}
	if p.FinishedAt, err = parseNullTime(finishedS); err != nil {
		return domain.Project{}, err
	}
	if p.CreatedAt, err = parseTime(createdS); err != nil {
		return domain.Project{}, err
	}
	if p.UpdatedAt, err = parseTime(updatedS); err != nil {
		return domain.Project{}, err
	}
	return p, nil
}

// --- Tasks ---

func (s *Store) CreateTask(ctx context.Context, t *domain.Task) error {
	if t.Status == "" {
		t.Status = domain.TaskOpen
	}
	if t.Priority == "" {
		t.Priority = domain.PriorityMedium
	}
	if err := t.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if t.ID == "" {
		t.ID = domain.NewID()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if t.Status == domain.TaskDone && t.CompletedAt == nil {
		t.CompletedAt = &now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tasks (id, project_id, title, description, status, priority, due_at, reminder_enabled, reminder_sent_at, position, completed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		t.ID, nullString(t.ProjectID), t.Title, t.Description, string(t.Status), string(t.Priority),
		formatNullTime(t.DueAt), boolToInt(t.ReminderEnabled), formatNullTime(t.ReminderSentAt),
		t.Position, formatNullTime(t.CompletedAt), formatTime(t.CreatedAt), formatTime(t.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

func (s *Store) GetTask(ctx context.Context, id string) (domain.Task, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, title, description, status, priority, due_at, reminder_enabled, reminder_sent_at, position, completed_at, created_at, updated_at
		FROM tasks WHERE id = ?
	`, id)
	return scanTask(row)
}

func (s *Store) ListTasksByProject(ctx context.Context, projectID string, includeClosed bool) ([]domain.Task, error) {
	query := `
		SELECT id, project_id, title, description, status, priority, due_at, reminder_enabled, reminder_sent_at, position, completed_at, created_at, updated_at
		FROM tasks WHERE project_id = ?`
	if !includeClosed {
		query += ` AND status NOT IN ('done', 'cancelled')`
	}
	query += ` ORDER BY position ASC, CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list tasks by project: %w", err)
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (s *Store) ListOpenTasks(ctx context.Context) ([]domain.Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, title, description, status, priority, due_at, reminder_enabled, reminder_sent_at, position, completed_at, created_at, updated_at
		FROM tasks
		WHERE status NOT IN ('done', 'cancelled')
		ORDER BY CASE WHEN due_at IS NULL THEN 1 ELSE 0 END, due_at ASC, created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list open tasks: %w", err)
	}
	defer rows.Close()
	return scanTasks(rows)
}

// ListDueReminderTasks returns open tasks with opt-in reminders enabled and not yet notified.
func (s *Store) ListDueReminderTasks(ctx context.Context) ([]domain.Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, title, description, status, priority, due_at, reminder_enabled, reminder_sent_at, position, completed_at, created_at, updated_at
		FROM tasks
		WHERE reminder_enabled = 1
		  AND reminder_sent_at IS NULL
		  AND due_at IS NOT NULL
		  AND status NOT IN ('done', 'cancelled')
		ORDER BY due_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list due reminder tasks: %w", err)
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (s *Store) UpdateTask(ctx context.Context, t *domain.Task) error {
	if err := t.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	t.UpdatedAt = now
	if t.Status == domain.TaskDone && t.CompletedAt == nil {
		t.CompletedAt = &now
	} else if t.IsOpen() {
		t.CompletedAt = nil
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE tasks
		SET project_id = ?, title = ?, description = ?, status = ?, priority = ?,
		    due_at = ?, reminder_enabled = ?, reminder_sent_at = ?, position = ?,
		    completed_at = ?, updated_at = ?
		WHERE id = ?
	`,
		nullString(t.ProjectID), t.Title, t.Description, string(t.Status), string(t.Priority),
		formatNullTime(t.DueAt), boolToInt(t.ReminderEnabled), formatNullTime(t.ReminderSentAt),
		t.Position, formatNullTime(t.CompletedAt), formatTime(t.UpdatedAt), t.ID,
	)
	if err != nil {
		return fmt.Errorf("update task %s: %w", t.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MarkTaskReminderSent(ctx context.Context, id string, sentAt time.Time) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE tasks SET reminder_sent_at = ?, updated_at = ? WHERE id = ?
	`, formatTime(sentAt), formatTime(time.Now().UTC()), id)
	if err != nil {
		return fmt.Errorf("mark task reminder sent: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTask(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanTask(r rowScanner) (domain.Task, error) {
	var (
		t                                    domain.Task
		projectID, dueS, remSentS, compS     sql.NullString
		status, priority, createdS, updatedS string
		remEnabled                           int
	)
	err := r.Scan(
		&t.ID, &projectID, &t.Title, &t.Description, &status, &priority,
		&dueS, &remEnabled, &remSentS, &t.Position, &compS, &createdS, &updatedS,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, ErrNotFound
		}
		return domain.Task{}, fmt.Errorf("scan task: %w", err)
	}
	t.ProjectID = projectID.String
	t.Status = domain.TaskStatus(status)
	t.Priority = domain.Priority(priority)
	t.ReminderEnabled = remEnabled != 0
	if t.DueAt, err = parseNullTime(dueS); err != nil {
		return domain.Task{}, err
	}
	if t.ReminderSentAt, err = parseNullTime(remSentS); err != nil {
		return domain.Task{}, err
	}
	if t.CompletedAt, err = parseNullTime(compS); err != nil {
		return domain.Task{}, err
	}
	if t.CreatedAt, err = parseTime(createdS); err != nil {
		return domain.Task{}, err
	}
	if t.UpdatedAt, err = parseTime(updatedS); err != nil {
		return domain.Task{}, err
	}
	return t, nil
}

func scanTasks(rows *sql.Rows) ([]domain.Task, error) {
	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
