package router

import (
	"net/http"

	"github.com/anshika/taskflow/internal/handlers"
	"github.com/anshika/taskflow/internal/middleware"
	"github.com/anshika/taskflow/internal/respond"
	"github.com/go-chi/chi/v5"
)

func New(api *handlers.API) http.Handler {
	r := chi.NewRouter()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", api.Register)
		r.Post("/login", api.Login)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.JWT(api.JWTSecret))
		r.Get("/projects", api.ListProjects)
		r.Post("/projects", api.CreateProject)
		r.Get("/projects/{id}", api.GetProject)
		r.Patch("/projects/{id}", api.PatchProject)
		r.Delete("/projects/{id}", api.DeleteProject)
		r.Get("/projects/{id}/stats", api.ProjectStats)
		r.Get("/projects/{id}/tasks", api.ListTasks)
		r.Post("/projects/{id}/tasks", api.CreateTask)
		r.Patch("/tasks/{id}", api.PatchTask)
		r.Delete("/tasks/{id}", api.DeleteTask)
	})

	return r
}
