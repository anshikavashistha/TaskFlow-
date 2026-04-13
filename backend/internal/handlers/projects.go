package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/anshika/taskflow/internal/db"
	"github.com/anshika/taskflow/internal/models"
	"github.com/anshika/taskflow/internal/respond"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type createProjectBody struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type patchProjectBody struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (a *API) ListProjects(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "invalid user")
		return
	}
	page, limit := parsePagination(r)
	items, total, err := db.ListProjects(r.Context(), a.Pool, db.ListProjectsParams{
		UserID: uid,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not list projects")
		return
	}
	respond.JSON(w, http.StatusOK, models.ListResponse[models.Project]{
		Data:       items,
		Pagination: models.Pagination{Page: page, Limit: limit, Total: total},
	})
}

func (a *API) CreateProject(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "invalid user")
		return
	}
	var body createProjectBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == "" {
		respond.Error(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(body.Name) > 255 {
		respond.Error(w, http.StatusBadRequest, "name too long (max 255 chars)")
		return
	}
	p, err := db.CreateProject(r.Context(), a.Pool, body.Name, body.Description, uid)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not create project")
		return
	}
	respond.JSON(w, http.StatusCreated, p)
}

func (a *API) GetProject(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "invalid user")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}
	ctx := r.Context()
	okAccess, err := db.CanAccessProject(ctx, a.Pool, id, uid)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not load project")
		return
	}
	if !okAccess {
		if _, err := db.GetProjectByID(ctx, a.Pool, id); errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "project not found")
			return
		}
		respond.Error(w, http.StatusForbidden, "forbidden")
		return
	}
	p, err := db.GetProjectByID(ctx, a.Pool, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "project not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "could not load project")
		return
	}
	tasks, _, err := db.ListTasks(ctx, a.Pool, db.ListTasksParams{
		ProjectID: id,
		Page:      1,
		Limit:     500,
	})
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not load tasks")
		return
	}
	type out struct {
		models.Project
		Tasks []models.Task `json:"tasks"`
	}
	respond.JSON(w, http.StatusOK, out{Project: p, Tasks: tasks})
}

func (a *API) PatchProject(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "invalid user")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}
	ctx := r.Context()
	owner, err := db.IsProjectOwner(ctx, a.Pool, id, uid)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not update project")
		return
	}
	if !owner {
		if _, err := db.GetProjectByID(ctx, a.Pool, id); errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "project not found")
			return
		}
		respond.Error(w, http.StatusForbidden, "only the owner can update this project")
		return
	}
	var body patchProjectBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name != nil && *body.Name == "" {
		respond.Error(w, http.StatusBadRequest, "name cannot be empty")
		return
	}
	if body.Name != nil && len(*body.Name) > 255 {
		respond.Error(w, http.StatusBadRequest, "name too long (max 255 chars)")
		return
	}
	p, err := db.UpdateProject(ctx, a.Pool, id, body.Name, body.Description)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "project not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "could not update project")
		return
	}
	respond.JSON(w, http.StatusOK, p)
}

func (a *API) DeleteProject(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "invalid user")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}
	ctx := r.Context()
	owner, err := db.IsProjectOwner(ctx, a.Pool, id, uid)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not delete project")
		return
	}
	if !owner {
		if _, err := db.GetProjectByID(ctx, a.Pool, id); errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "project not found")
			return
		}
		respond.Error(w, http.StatusForbidden, "only the owner can delete this project")
		return
	}
	if err := db.DeleteProject(ctx, a.Pool, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "project not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "could not delete project")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) ProjectStats(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "invalid user")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}
	ctx := r.Context()
	okAccess, err := db.CanAccessProject(ctx, a.Pool, id, uid)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not load stats")
		return
	}
	if !okAccess {
		if _, err := db.GetProjectByID(ctx, a.Pool, id); errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "project not found")
			return
		}
		respond.Error(w, http.StatusForbidden, "forbidden")
		return
	}
	stats, err := db.ProjectStats(ctx, a.Pool, id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not load stats")
		return
	}
	respond.JSON(w, http.StatusOK, stats)
}
