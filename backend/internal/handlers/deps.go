package handlers

import (
	"net/http"
	"strconv"

	"github.com/anshika/taskflow/internal/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type API struct {
	Pool      *pgxpool.Pool
	JWTSecret string
}

func userID(r *http.Request) (uuid.UUID, bool) {
	v := r.Context().Value(middleware.UserIDKey)
	s, ok := v.(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	return id, err == nil
}

func parsePagination(r *http.Request) (page, limit int) {
	page = 1
	limit = 20
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}
