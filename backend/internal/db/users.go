package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/anshika/taskflow/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateUser(ctx context.Context, pool *pgxpool.Pool, name, email, passwordHash string) (models.User, error) {
	const q = `
		INSERT INTO users (name, email, password)
		VALUES ($1, $2, $3)
		RETURNING id, name, email, created_at
	`
	var u models.User
	err := pool.QueryRow(ctx, q, name, email, passwordHash).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if err != nil {
		return models.User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

func UserByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (models.User, string, error) {
	const q = `SELECT id, name, email, password, created_at FROM users WHERE email = $1`
	var u models.User
	var hash string
	err := pool.QueryRow(ctx, q, email).Scan(&u.ID, &u.Name, &u.Email, &hash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, "", pgx.ErrNoRows
	}
	if err != nil {
		return models.User{}, "", fmt.Errorf("user by email: %w", err)
	}
	return u, hash, nil
}

func UserExistsByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (bool, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT 1 FROM users WHERE email = $1 LIMIT 1`, email).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func UserByID(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (models.User, error) {
	const q = `SELECT id, name, email, created_at FROM users WHERE id = $1`
	var u models.User
	err := pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if err != nil {
		return models.User{}, err
	}
	return u, nil
}
