package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anshika/taskflow/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidAssigneeID = errors.New("invalid assignee id")
	ErrInvalidDueDate    = errors.New("invalid due_date")
)

type ListTasksParams struct {
	ProjectID      uuid.UUID
	StatusFilter   *string
	AssigneeFilter *string
	Page           int
	Limit          int
}

func ListTasks(ctx context.Context, pool *pgxpool.Pool, p ListTasksParams) ([]models.Task, int, error) {
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

	where := []string{"project_id = $1"}
	args := []any{p.ProjectID}
	n := 2
	if p.StatusFilter != nil && *p.StatusFilter != "" {
		where = append(where, fmt.Sprintf("status = $%d::task_status", n))
		args = append(args, *p.StatusFilter)
		n++
	}
	if p.AssigneeFilter != nil && *p.AssigneeFilter != "" {
		assigneeUUID, err := uuid.Parse(*p.AssigneeFilter)
		if err != nil {
			return nil, 0, ErrInvalidAssigneeID
		}
		where = append(where, fmt.Sprintf("assignee_id = $%d", n))
		args = append(args, assigneeUUID)
		n++
	}
	wclause := strings.Join(where, " AND ")
	limitArg := n
	offsetArg := n + 1

	countQ := "SELECT COUNT(*) FROM tasks WHERE " + wclause
	var total int
	if err := pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQ := fmt.Sprintf(`
		SELECT id, title, description, status::text, priority::text, project_id, assignee_id,
		       due_date::text, created_at, updated_at
		FROM tasks
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, wclause, limitArg, offsetArg)
	args = append(args, p.Limit, offset)

	rows, err := pool.Query(ctx, dataQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []models.Task
	for rows.Next() {
		tk, err := scanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, tk)
	}
	return out, total, rows.Err()
}

func scanTask(row interface{ Scan(dest ...any) error }) (models.Task, error) {
	var t models.Task
	var status, priority string
	var due *string
	err := row.Scan(
		&t.ID, &t.Title, &t.Description, &status, &priority, &t.ProjectID, &t.AssigneeID,
		&due, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return models.Task{}, err
	}
	t.Status = models.TaskStatus(status)
	t.Priority = models.TaskPriority(priority)
	if due != nil && *due != "" {
		t.DueDate = due
	}
	return t, nil
}

func GetTaskByID(ctx context.Context, pool *pgxpool.Pool, taskID uuid.UUID) (models.Task, error) {
	const q = `
		SELECT id, title, description, status::text, priority::text, project_id, assignee_id,
		       due_date::text, created_at, updated_at
		FROM tasks WHERE id = $1
	`
	return scanTask(pool.QueryRow(ctx, q, taskID))
}

type TaskInsert struct {
	Title       string
	Description *string
	Status      models.TaskStatus
	Priority    models.TaskPriority
	ProjectID   uuid.UUID
	AssigneeID  *uuid.UUID
	DueDate     *string
}

func CreateTask(ctx context.Context, pool *pgxpool.Pool, in TaskInsert) (models.Task, error) {
	const q = `
		INSERT INTO tasks (title, description, status, priority, project_id, assignee_id, due_date)
		VALUES ($1, $2, $3::task_status, $4::task_priority, $5, $6, $7)
		RETURNING id, title, description, status::text, priority::text, project_id, assignee_id,
		          due_date::text, created_at, updated_at
	`
	var due any
	if in.DueDate != nil && *in.DueDate != "" {
		d, err := time.Parse("2006-01-02", *in.DueDate)
		if err != nil {
			return models.Task{}, ErrInvalidDueDate
		}
		due = d
	}
	return scanTask(pool.QueryRow(ctx, q,
		in.Title, in.Description, string(in.Status), string(in.Priority), in.ProjectID, in.AssigneeID, due,
	))
}

type TaskUpdate struct {
	Title       *string
	Description *string
	Status      *string
	Priority    *string
	AssigneeID  *uuid.UUID
	DueDate     *string
}

func UpdateTask(ctx context.Context, pool *pgxpool.Pool, taskID uuid.UUID, u TaskUpdate) (models.Task, error) {
	var sets []string
	var args []any
	argPos := 1
	if u.Title != nil {
		sets = append(sets, fmt.Sprintf("title = $%d", argPos))
		args = append(args, *u.Title)
		argPos++
	}
	if u.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argPos))
		args = append(args, *u.Description)
		argPos++
	}
	if u.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d::task_status", argPos))
		args = append(args, *u.Status)
		argPos++
	}
	if u.Priority != nil {
		sets = append(sets, fmt.Sprintf("priority = $%d::task_priority", argPos))
		args = append(args, *u.Priority)
		argPos++
	}
	if u.AssigneeID != nil {
		if *u.AssigneeID == uuid.Nil {
			sets = append(sets, "assignee_id = NULL")
		} else {
			sets = append(sets, fmt.Sprintf("assignee_id = $%d", argPos))
			args = append(args, *u.AssigneeID)
			argPos++
		}
	}
	if u.DueDate != nil {
		if *u.DueDate == "" {
			sets = append(sets, "due_date = NULL")
		} else {
			d, err := time.Parse("2006-01-02", *u.DueDate)
			if err != nil {
				return models.Task{}, ErrInvalidDueDate
			}
			sets = append(sets, fmt.Sprintf("due_date = $%d", argPos))
			args = append(args, d)
			argPos++
		}
	}
	if len(sets) == 0 {
		return GetTaskByID(ctx, pool, taskID)
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, taskID)
	q := fmt.Sprintf(
		`UPDATE tasks SET %s WHERE id = $%d
		 RETURNING id, title, description, status::text, priority::text, project_id, assignee_id,
		             due_date::text, created_at, updated_at`,
		strings.Join(sets, ", "),
		argPos,
	)
	return scanTask(pool.QueryRow(ctx, q, args...))
}

func DeleteTask(ctx context.Context, pool *pgxpool.Pool, taskID uuid.UUID) error {
	tag, err := pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func TaskProjectID(ctx context.Context, pool *pgxpool.Pool, taskID uuid.UUID) (uuid.UUID, error) {
	var pid uuid.UUID
	err := pool.QueryRow(ctx, `SELECT project_id FROM tasks WHERE id = $1`, taskID).Scan(&pid)
	return pid, err
}

func IsTaskAssignee(ctx context.Context, pool *pgxpool.Pool, taskID, userID uuid.UUID) (bool, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT 1 FROM tasks WHERE id = $1 AND assignee_id = $2`, taskID, userID).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
