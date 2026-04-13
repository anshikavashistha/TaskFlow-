package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/anshika/taskflow/internal/auth"
	"github.com/anshika/taskflow/internal/respond"
)

type ctxKey string

const UserIDKey ctxKey = "user_id"

func JWT(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := strings.TrimSpace(r.Header.Get("Authorization"))
			parts := strings.SplitN(h, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				respond.Error(w, http.StatusUnauthorized, "missing or invalid authorization")
				return
			}
			raw := strings.TrimSpace(parts[1])
			if raw == "" {
				respond.Error(w, http.StatusUnauthorized, "missing token")
				return
			}
			claims, err := auth.ValidateToken(raw, secret)
			if err != nil {
				respond.Error(w, http.StatusUnauthorized, "invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
