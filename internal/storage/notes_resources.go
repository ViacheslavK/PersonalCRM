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

// --- Notes ---

func (s *Store) CreateNote(ctx context.Context, n *domain.Note) error {
	if err := n.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if n.ID == "" {
		n.ID = domain.NewID()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	n.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notes (id, title, content, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, n.ID, n.Title, n.Content, n.Source, formatTime(n.CreatedAt), formatTime(n.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert note: %w", err)
	}
	return nil
}

func (s *Store) GetNote(ctx context.Context, id string) (domain.Note, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, content, source, created_at, updated_at
		FROM notes WHERE id = ?
	`, id)
	return scanNote(row)
}

func (s *Store) ListNotes(ctx context.Context, queryText string) ([]domain.Note, error) {
	query := `SELECT id, title, content, source, created_at, updated_at FROM notes`
	var args []any
	if q := strings.TrimSpace(queryText); q != "" {
		like := "%" + q + "%"
		query += ` WHERE title LIKE ? OR content LIKE ? OR source LIKE ?`
		args = append(args, like, like, like)
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var out []domain.Note
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) UpdateNote(ctx context.Context, n *domain.Note) error {
	if err := n.Validate(); err != nil {
		return err
	}
	n.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE notes SET title = ?, content = ?, source = ?, updated_at = ? WHERE id = ?
	`, n.Title, n.Content, n.Source, formatTime(n.UpdatedAt), n.ID)
	if err != nil {
		return fmt.Errorf("update note %s: %w", n.ID, err)
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteNote(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete note %s: %w", id, err)
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return ErrNotFound
	}
	if err := cleanupPolymorphicTx(ctx, tx, domain.EntityNote, id); err != nil {
		return err
	}
	return tx.Commit()
}

func scanNote(r rowScanner) (domain.Note, error) {
	var (
		n                  domain.Note
		createdS, updatedS string
	)
	err := r.Scan(&n.ID, &n.Title, &n.Content, &n.Source, &createdS, &updatedS)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Note{}, ErrNotFound
		}
		return domain.Note{}, fmt.Errorf("scan note: %w", err)
	}
	if n.CreatedAt, err = parseTime(createdS); err != nil {
		return domain.Note{}, err
	}
	if n.UpdatedAt, err = parseTime(updatedS); err != nil {
		return domain.Note{}, err
	}
	return n, nil
}

// --- Resources / Links ---

func (s *Store) CreateResource(ctx context.Context, r *domain.Resource) error {
	if r.ResourceType == "" {
		r.ResourceType = domain.ResourceLink
	}
	if r.Status == "" {
		r.Status = domain.ResourceReference
	}
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

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO resources (id, title, url_or_path, resource_type, status, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		r.ID, r.Title, r.URLOrPath, string(r.ResourceType), string(r.Status),
		r.Notes, formatTime(r.CreatedAt), formatTime(r.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert resource: %w", err)
	}
	return nil
}

func (s *Store) GetResource(ctx context.Context, id string) (domain.Resource, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, url_or_path, resource_type, status, notes, created_at, updated_at
		FROM resources WHERE id = ?
	`, id)
	return scanResource(row)
}

func (s *Store) ListResources(ctx context.Context, queryText string) ([]domain.Resource, error) {
	query := `SELECT id, title, url_or_path, resource_type, status, notes, created_at, updated_at FROM resources`
	var args []any
	if q := strings.TrimSpace(queryText); q != "" {
		like := "%" + q + "%"
		query += ` WHERE title LIKE ? OR url_or_path LIKE ? OR notes LIKE ?`
		args = append(args, like, like, like)
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()

	var out []domain.Resource
	for rows.Next() {
		res, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

func (s *Store) UpdateResource(ctx context.Context, r *domain.Resource) error {
	if err := r.Validate(); err != nil {
		return err
	}
	r.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE resources
		SET title = ?, url_or_path = ?, resource_type = ?, status = ?, notes = ?, updated_at = ?
		WHERE id = ?
	`, r.Title, r.URLOrPath, string(r.ResourceType), string(r.Status), r.Notes, formatTime(r.UpdatedAt), r.ID)
	if err != nil {
		return fmt.Errorf("update resource %s: %w", r.ID, err)
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteResource(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete resource %s: %w", id, err)
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return ErrNotFound
	}
	if err := cleanupPolymorphicTx(ctx, tx, domain.EntityResource, id); err != nil {
		return err
	}
	return tx.Commit()
}

func scanResource(row rowScanner) (domain.Resource, error) {
	var (
		r                                    domain.Resource
		resType, status, createdS, updatedS string
	)
	err := row.Scan(&r.ID, &r.Title, &r.URLOrPath, &resType, &status, &r.Notes, &createdS, &updatedS)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Resource{}, ErrNotFound
		}
		return domain.Resource{}, fmt.Errorf("scan resource: %w", err)
	}
	r.ResourceType = domain.ResourceType(resType)
	r.Status = domain.ResourceStatus(status)
	if r.CreatedAt, err = parseTime(createdS); err != nil {
		return domain.Resource{}, err
	}
	if r.UpdatedAt, err = parseTime(updatedS); err != nil {
		return domain.Resource{}, err
	}
	return r, nil
}
