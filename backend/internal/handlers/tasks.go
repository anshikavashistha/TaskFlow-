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

func (a *API) ListTasks(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "invalid user")
		return
	}
	pid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}
	ctx := r.Context()
	okAccess, err := db.CanAccessProject(ctx, a.Pool, pid, uid)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not list tasks")
		return
	}
	if !okAccess {
		if _, err := db.GetProjectByID(ctx, a.Pool, pid); errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "project not found")
			return
		}
		respond.Error(w, http.StatusForbidden, "forbidden")
		return
	}

	q := r.URL.Query()
	var statusPtr, assigneePtr *string
	if v := q.Get("status"); v != "" {
		statusPtr = &v
	}
	if v := q.Get("assignee"); v != "" {
		assigneePtr = &v
	}
	page, limit := parsePagination(r)
	items, total, err := db.ListTasks(ctx, a.Pool, db.ListTasksParams{
		ProjectID:      pid,
		StatusFilter:   statusPtr,
		AssigneeFilter: assigneePtr,
		Page:           page,
		Limit:          limit,
	})
	if err != nil {
		if errors.Is(err, db.ErrInvalidAssigneeID) {
			respond.Error(w, http.StatusBadRequest, "invalid assignee id")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "could not list tasks")
		return
	}
	respond.JSON(w, http.StatusOK, models.ListResponse[models.Task]{
		Data:       items,
		Pagination: models.Pagination{Page: page, Limit: limit, Total: total},
	})
}

type createTaskBody struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
}

func (a *API) CreateTask(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "invalid user")
		return
	}
	pid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}
	ctx := r.Context()
	owner, err := db.IsProjectOwner(ctx, a.Pool, pid, uid)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not create task")
		return
	}
	if !owner {
		if _, err := db.GetProjectByID(ctx, a.Pool, pid); errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "project not found")
			return
		}
		respond.Error(w, http.StatusForbidden, "only the project owner can create tasks")
		return
	}

	var body createTaskBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Title == "" {
		respond.Error(w, http.StatusBadRequest, "title is required")
		return
	}
	st := models.TaskTodo
	if body.Status != nil {
		st = models.TaskStatus(*body.Status)
	}
	pr := models.PriorityMedium
	if body.Priority != nil {
		pr = models.TaskPriority(*body.Priority)
	}
	var assignee *uuid.UUID
	if body.AssigneeID != nil && *body.AssigneeID != "" {
		aid, err := uuid.Parse(*body.AssigneeID)
		if err != nil {
			respond.Error(w, http.StatusBadRequest, "invalid assignee_id")
			return
		}
		assignee = &aid
	}
	in := db.TaskInsert{
		Title:       body.Title,
		Description: body.Description,
		Status:      st,
		Priority:    pr,
		ProjectID:   pid,
		AssigneeID:  assignee,
		DueDate:     body.DueDate,
	}
	tk, err := db.CreateTask(ctx, a.Pool, in)
	if err != nil {
		if errors.Is(err, db.ErrInvalidDueDate) {
			respond.Error(w, http.StatusBadRequest, "invalid due_date")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "could not create task")
		return
	}
	respond.JSON(w, http.StatusCreated, tk)
}

type patchTaskBody struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
}

func (a *API) PatchTask(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "invalid user")
		return
	}
	tid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid task id")
		return
	}
	ctx := r.Context()
	pid, err := db.TaskProjectID(ctx, a.Pool, tid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "task not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "could not update task")
		return
	}
	owner, _ := db.IsProjectOwner(ctx, a.Pool, pid, uid)
	assignee, _ := db.IsTaskAssignee(ctx, a.Pool, tid, uid)
	if !owner && !assignee {
		respond.Error(w, http.StatusForbidden, "forbidden")
		return
	}

	var body patchTaskBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	up := db.TaskUpdate{
		Title:       body.Title,
		Description: body.Description,
		Status:      body.Status,
		Priority:    body.Priority,
		DueDate:     body.DueDate,
	}
	if body.AssigneeID != nil {
		if *body.AssigneeID == "" {
			z := uuid.Nil
			up.AssigneeID = &z
		} else {
			aid, err := uuid.Parse(*body.AssigneeID)
			if err != nil {
				respond.Error(w, http.StatusBadRequest, "invalid assignee_id")
				return
			}
			up.AssigneeID = &aid
		}
	}
	tk, err := db.UpdateTask(ctx, a.Pool, tid, up)
	if err != nil {
		if errors.Is(err, db.ErrInvalidDueDate) {
			respond.Error(w, http.StatusBadRequest, "invalid due_date")
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "task not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "could not update task")
		return
	}
	respond.JSON(w, http.StatusOK, tk)
}

func (a *API) DeleteTask(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "invalid user")
		return
	}
	tid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid task id")
		return
	}
	ctx := r.Context()
	pid, err := db.TaskProjectID(ctx, a.Pool, tid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "task not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "could not delete task")
		return
	}
	owner, _ := db.IsProjectOwner(ctx, a.Pool, pid, uid)
	assignee, _ := db.IsTaskAssignee(ctx, a.Pool, tid, uid)
	if !owner && !assignee {
		respond.Error(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := db.DeleteTask(ctx, a.Pool, tid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "task not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "could not delete task")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
