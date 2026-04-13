package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anshika/taskflow/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func IsProjectOwner(ctx context.Context, pool *pgxpool.Pool, projectID, userID uuid.UUID) (bool, error) {
	const q = `SELECT 1 FROM projects WHERE id = $1 AND owner_id = $2`
	var n int
	err := pool.QueryRow(ctx, q, projectID, userID).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func CanAccessProject(ctx context.Context, pool *pgxpool.Pool, projectID, userID uuid.UUID) (bool, error) {
	const q = `
		SELECT 1 FROM projects p
		WHERE p.id = $1
		  AND (
		    p.owner_id = $2
		    OR EXISTS (
		      SELECT 1 FROM tasks t
		      WHERE t.project_id = p.id AND t.assignee_id = $2
		    )
		  )
		LIMIT 1
	`
	var n int
	err := pool.QueryRow(ctx, q, projectID, userID).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetProjectByID(ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID) (models.Project, error) {
	const q = `
		SELECT id, name, description, owner_id, created_at
		FROM projects WHERE id = $1
	`
	var p models.Project
	err := pool.QueryRow(ctx, q, projectID).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt)
	if err != nil {
		return models.Project{}, err
	}
	return p, nil
}

type ListProjectsParams struct {
	UserID uuid.UUID
	Page   int
	Limit  int
}

func ListProjects(ctx context.Context, pool *pgxpool.Pool, p ListProjectsParams) ([]models.Project, int, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 {
		p.Limit = 20
	}
	if p.Limit > 100 {
		p.Limit = 100
	}
	offset := (p.Page - 1) * p.Limit

	baseFrom := `
		FROM projects p
		WHERE p.owner_id = $1
		   OR EXISTS (
		     SELECT 1 FROM tasks t
		     WHERE t.project_id = p.id AND t.assignee_id = $1
		   )
	`
	countQ := "SELECT COUNT(DISTINCT p.id) " + baseFrom
	var total int
	if err := pool.QueryRow(ctx, countQ, p.UserID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}

	dataQ := `
		SELECT DISTINCT p.id, p.name, p.description, p.owner_id, p.created_at
	` + baseFrom + `
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := pool.Query(ctx, dataQ, p.UserID, p.Limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var out []models.Project
	for rows.Next() {
		var pr models.Project
		if err := rows.Scan(&pr.ID, &pr.Name, &pr.Description, &pr.OwnerID, &pr.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, pr)
	}
	return out, total, rows.Err()
}

func CreateProject(ctx context.Context, pool *pgxpool.Pool, name string, description *string, ownerID uuid.UUID) (models.Project, error) {
	const q = `
		INSERT INTO projects (name, description, owner_id)
		VALUES ($1, $2, $3)
		RETURNING id, name, description, owner_id, created_at
	`
	var p models.Project
	err := pool.QueryRow(ctx, q, name, description, ownerID).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt)
	if err != nil {
		return models.Project{}, err
	}
	return p, nil
}

func UpdateProject(ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID, name *string, description *string) (models.Project, error) {
	var sets []string
	var args []any
	argPos := 1
	if name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argPos))
		args = append(args, *name)
		argPos++
	}
	if description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argPos))
		args = append(args, *description)
		argPos++
	}
	if len(sets) == 0 {
		return GetProjectByID(ctx, pool, projectID)
	}
	args = append(args, projectID)
	q := fmt.Sprintf(
		`UPDATE projects SET %s WHERE id = $%d RETURNING id, name, description, owner_id, created_at`,
		strings.Join(sets, ", "),
		argPos,
	)
	var p models.Project
	err := pool.QueryRow(ctx, q, args...).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt)
	if err != nil {
		return models.Project{}, err
	}
	return p, nil
}

func DeleteProject(ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID) error {
	tag, err := pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, projectID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func ProjectStats(ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID) (models.ProjectStats, error) {
	const statusQ = `
		SELECT status::text, COUNT(*)::int
		FROM tasks
		WHERE project_id = $1
		GROUP BY status
	`
	rows, err := pool.Query(ctx, statusQ, projectID)
	if err != nil {
		return models.ProjectStats{}, err
	}
	defer rows.Close()

	byStatus := map[string]int{
		string(models.TaskTodo):       0,
		string(models.TaskInProgress): 0,
		string(models.TaskDone):       0,
	}
	for rows.Next() {
		var st string
		var c int
		if err := rows.Scan(&st, &c); err != nil {
			return models.ProjectStats{}, err
		}
		byStatus[st] = c
	}
	if err := rows.Err(); err != nil {
		return models.ProjectStats{}, err
	}

	const assigneeQ = `
		SELECT t.assignee_id, u.name, COUNT(*)::int
		FROM tasks t
		LEFT JOIN users u ON u.id = t.assignee_id
		WHERE t.project_id = $1
		GROUP BY t.assignee_id, u.name
		ORDER BY COUNT(*) DESC
	`
	arows, err := pool.Query(ctx, assigneeQ, projectID)
	if err != nil {
		return models.ProjectStats{}, err
	}
	defer arows.Close()

	var byAssignee []models.AssigneeStatRow
	for arows.Next() {
		var row models.AssigneeStatRow
		if err := arows.Scan(&row.AssigneeID, &row.Name, &row.Count); err != nil {
			return models.ProjectStats{}, err
		}
		byAssignee = append(byAssignee, row)
	}
	return models.ProjectStats{ByStatus: byStatus, ByAssignee: byAssignee}, arows.Err()
}
