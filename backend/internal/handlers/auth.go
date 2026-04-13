package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/anshika/taskflow/internal/auth"
	"github.com/anshika/taskflow/internal/db"
	"github.com/anshika/taskflow/internal/respond"
	"github.com/jackc/pgx/v5"
)

type registerBody struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  interface{} `json:"user"`
}

func (a *API) Register(w http.ResponseWriter, r *http.Request) {
	var body registerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Name == "" || body.Email == "" || body.Password == "" {
		respond.Error(w, http.StatusBadRequest, "name, email, and password are required")
		return
	}

	ctx := r.Context()
	taken, err := db.UserExistsByEmail(ctx, a.Pool, body.Email)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not create user")
		return
	}
	if taken {
		respond.Error(w, http.StatusBadRequest, "email already registered")
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not create user")
		return
	}

	u, err := db.CreateUser(ctx, a.Pool, body.Name, body.Email, hash)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not create user")
		return
	}

	token, err := auth.GenerateToken(u.ID, u.Email, a.JWTSecret)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not create user")
		return
	}

	respond.JSON(w, http.StatusCreated, authResponse{Token: token, User: u})
}

func (a *API) Login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Email == "" || body.Password == "" {
		respond.Error(w, http.StatusBadRequest, "email and password are required")
		return
	}

	ctx := r.Context()
	u, hash, err := db.UserByEmail(ctx, a.Pool, body.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not login")
		return
	}
	if !auth.CheckPassword(hash, body.Password) {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := auth.GenerateToken(u.ID, u.Email, a.JWTSecret)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not login")
		return
	}

	respond.JSON(w, http.StatusOK, authResponse{Token: token, User: u})
}
