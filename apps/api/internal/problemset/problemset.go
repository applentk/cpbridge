package problemset

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cpbridge/api/internal/idgen"
	"github.com/cpbridge/api/internal/problem"
)

type Visibility string

const (
	Public   Visibility = "PUBLIC"
	Unlisted Visibility = "UNLISTED"
	Private  Visibility = "PRIVATE"
)

type ProblemSetItem struct {
	ProblemSetID string           `json:"problemSetId"`
	ProblemID    string           `json:"problemId"`
	Position     int              `json:"position"`
	Problem      *problem.Problem `json:"problem,omitempty"`
}

type ProblemSet struct {
	ID            string           `json:"id"`
	OwnerID       string           `json:"ownerId"`
	OwnerUsername string           `json:"ownerUsername,omitempty"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	Visibility    Visibility       `json:"visibility"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
	Items         []ProblemSetItem `json:"items,omitempty"`
	ProblemCount  int              `json:"problemCount"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(ctx context.Context, ownerID, name, description string, visibility Visibility) (*ProblemSet, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("problem set name is required")
	}
	if visibility == "" {
		visibility = Public
	}

	set := &ProblemSet{
		ID:          idgen.New(idgen.PrefixProblemSet),
		OwnerID:     ownerID,
		Name:        name,
		Description: description,
		Visibility:  visibility,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	query := `
		INSERT INTO problem_sets (id, owner_id, name, description, visibility, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.ExecContext(ctx, query, set.ID, set.OwnerID, set.Name, set.Description, set.Visibility, set.CreatedAt, set.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create problem set: %w", err)
	}

	return set, nil
}

func (s *Service) GetByID(ctx context.Context, id string, requestingUserID string) (*ProblemSet, error) {
	query := `
		SELECT ps.id, ps.owner_id, u.username, ps.name, ps.description, ps.visibility, ps.created_at, ps.updated_at
		FROM problem_sets ps
		JOIN users u ON ps.owner_id = u.id
		WHERE ps.id = $1
	`
	var set ProblemSet
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&set.ID, &set.OwnerID, &set.OwnerUsername, &set.Name, &set.Description, &set.Visibility, &set.CreatedAt, &set.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("problem set not found")
		}
		return nil, err
	}

	if set.Visibility == Private && set.OwnerID != requestingUserID {
		return nil, errors.New("unauthorized to view this private problem set")
	}

	itemsQuery := `
		SELECT psi.problem_set_id, psi.problem_id, psi.position,
		       p.id, p.platform, p.external_id, p.title, p.url, p.difficulty, p.tags, p.metadata, p.created_at, p.updated_at
		FROM problem_set_items psi
		JOIN problems p ON psi.problem_id = p.id
		WHERE psi.problem_set_id = $1
		ORDER BY psi.position ASC
	`
	rows, err := s.db.QueryContext(ctx, itemsQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ProblemSetItem
	for rows.Next() {
		var item ProblemSetItem
		var p problem.Problem
		var tagsJSON, metaJSON []byte

		err := rows.Scan(
			&item.ProblemSetID, &item.ProblemID, &item.Position,
			&p.ID, &p.Platform, &p.ExternalID, &p.Title, &p.URL, &p.Difficulty, &tagsJSON, &metaJSON, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		_ = json.Unmarshal(tagsJSON, &p.Tags)
		_ = json.Unmarshal(metaJSON, &p.Metadata)
		item.Problem = &p
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if items == nil {
		items = []ProblemSetItem{}
	}

	set.Items = items
	set.ProblemCount = len(items)
	return &set, nil
}

func (s *Service) List(ctx context.Context, ownerIDFilter string, requestingUserID string) ([]ProblemSet, error) {
	var query string
	var args []any

	if ownerIDFilter != "" {
		if ownerIDFilter == requestingUserID {
			query = `
				SELECT ps.id, ps.owner_id, u.username, ps.name, ps.description, ps.visibility, ps.created_at, ps.updated_at,
				       (SELECT COUNT(*) FROM problem_set_items psi WHERE psi.problem_set_id = ps.id) as problem_count
				FROM problem_sets ps
				JOIN users u ON ps.owner_id = u.id
				WHERE ps.owner_id = $1
				ORDER BY ps.created_at DESC
			`
			args = append(args, ownerIDFilter)
		} else {
			query = `
				SELECT ps.id, ps.owner_id, u.username, ps.name, ps.description, ps.visibility, ps.created_at, ps.updated_at,
				       (SELECT COUNT(*) FROM problem_set_items psi WHERE psi.problem_set_id = ps.id) as problem_count
				FROM problem_sets ps
				JOIN users u ON ps.owner_id = u.id
				WHERE ps.owner_id = $1 AND ps.visibility = 'PUBLIC'
				ORDER BY ps.created_at DESC
			`
			args = append(args, ownerIDFilter)
		}
	} else {
		query = `
			SELECT ps.id, ps.owner_id, u.username, ps.name, ps.description, ps.visibility, ps.created_at, ps.updated_at,
			       (SELECT COUNT(*) FROM problem_set_items psi WHERE psi.problem_set_id = ps.id) as problem_count
			FROM problem_sets ps
			JOIN users u ON ps.owner_id = u.id
			WHERE ps.visibility = 'PUBLIC' OR ps.owner_id = $1
			ORDER BY ps.created_at DESC
		`
		args = append(args, requestingUserID)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sets []ProblemSet
	for rows.Next() {
		var set ProblemSet
		err := rows.Scan(
			&set.ID, &set.OwnerID, &set.OwnerUsername, &set.Name, &set.Description, &set.Visibility, &set.CreatedAt, &set.UpdatedAt, &set.ProblemCount,
		)
		if err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if sets == nil {
		sets = []ProblemSet{}
	}

	return sets, nil
}

func (s *Service) Update(ctx context.Context, id, ownerID string, name, description *string, visibility *Visibility) (*ProblemSet, error) {
	set, err := s.GetByID(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}
	if set.OwnerID != ownerID {
		return nil, errors.New("unauthorized to modify this problem set")
	}

	if name != nil && strings.TrimSpace(*name) != "" {
		set.Name = strings.TrimSpace(*name)
	}
	if description != nil {
		set.Description = *description
	}
	if visibility != nil {
		set.Visibility = *visibility
	}
	set.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE problem_sets
		SET name = $1, description = $2, visibility = $3, updated_at = $4
		WHERE id = $5
	`
	_, err = s.db.ExecContext(ctx, query, set.Name, set.Description, set.Visibility, set.UpdatedAt, set.ID)
	if err != nil {
		return nil, err
	}

	return set, nil
}

func (s *Service) Delete(ctx context.Context, id, ownerID string) error {
	set, err := s.GetByID(ctx, id, ownerID)
	if err != nil {
		return err
	}
	if set.OwnerID != ownerID {
		return errors.New("unauthorized to delete this problem set")
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM problem_sets WHERE id = $1`, id)
	return err
}

func (s *Service) AddProblem(ctx context.Context, setID, ownerID, problemID string, position *int) error {
	set, err := s.GetByID(ctx, setID, ownerID)
	if err != nil {
		return err
	}
	if set.OwnerID != ownerID {
		return errors.New("unauthorized")
	}

	pos := len(set.Items)
	if position != nil && *position >= 0 {
		pos = *position
	}

	query := `
		INSERT INTO problem_set_items (problem_set_id, problem_id, position)
		VALUES ($1, $2, $3)
		ON CONFLICT (problem_set_id, problem_id) DO UPDATE SET position = EXCLUDED.position
	`
	_, err = s.db.ExecContext(ctx, query, setID, problemID, pos)
	return err
}

func (s *Service) RemoveProblem(ctx context.Context, setID, ownerID, problemID string) error {
	set, err := s.GetByID(ctx, setID, ownerID)
	if err != nil {
		return err
	}
	if set.OwnerID != ownerID {
		return errors.New("unauthorized")
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM problem_set_items WHERE problem_set_id = $1 AND problem_id = $2`, setID, problemID)
	return err
}

func (s *Service) Reorder(ctx context.Context, setID, ownerID string, problemIDs []string) error {
	set, err := s.GetByID(ctx, setID, ownerID)
	if err != nil {
		return err
	}
	if set.OwnerID != ownerID {
		return errors.New("unauthorized")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, pid := range problemIDs {
		_, err := tx.ExecContext(ctx, `
			UPDATE problem_set_items
			SET position = $1
			WHERE problem_set_id = $2 AND problem_id = $3
		`, i, setID, pid)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
