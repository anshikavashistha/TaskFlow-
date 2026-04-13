package db

import (
	"context"
	"fmt"

	"github.com/anshika/taskflow/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	seedUserID    = "00000000-0000-0000-0000-000000000001"
	seedProjectID = "00000000-0000-0000-0000-000000000002"
)

// RunSeed inserts a demo user, project, and tasks if the seed user email is unused.
func RunSeed(ctx context.Context, pool *pgxpool.Pool) error {
	const email = "test@example.com"
	exists, err := UserExistsByEmail(ctx, pool, email)
	if err != nil {
		return fmt.Errorf("seed check: %w", err)
	}
	if exists {
		return nil
	}

	hash, err := auth.HashPassword("password123")
	if err != nil {
		return err
	}

	userID, _ := uuid.Parse(seedUserID)
	projectID, _ := uuid.Parse(seedProjectID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, name, email, password)
		VALUES ($1, 'Test User', $2, $3)
	`, userID, email, hash); err != nil {
		return fmt.Errorf("seed user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO projects (id, name, description, owner_id)
		VALUES ($1, 'Demo Project', 'Seeded for local development', $2)
	`, projectID, userID); err != nil {
		return fmt.Errorf("seed project: %w", err)
	}

	tasks := []struct {
		title  string
		status string
	}{
		{"Write API docs", "todo"},
		{"Implement auth middleware", "in_progress"},
		{"Ship TaskFlow", "done"},
	}
	for _, t := range tasks {
		if _, err := tx.Exec(ctx, `
			INSERT INTO tasks (title, status, priority, project_id, assignee_id)
			VALUES ($1, $2::task_status, 'medium'::task_priority, $3, $4)
		`, t.title, t.status, projectID, userID); err != nil {
			return fmt.Errorf("seed task: %w", err)
		}
	}

	return tx.Commit(ctx)
}
