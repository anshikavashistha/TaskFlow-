package models

import "time"

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type TaskStatus string

const (
	TaskTodo       TaskStatus = "todo"
	TaskInProgress TaskStatus = "in_progress"
	TaskDone       TaskStatus = "done"
)

type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

type Task struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description *string        `json:"description,omitempty"`
	Status      TaskStatus     `json:"status"`
	Priority    TaskPriority   `json:"priority"`
	ProjectID   string         `json:"project_id"`
	AssigneeID  *string        `json:"assignee_id,omitempty"`
	DueDate     *string        `json:"due_date,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Pagination struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}

type ListResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type ProjectStats struct {
	ByStatus     map[string]int       `json:"by_status"`
	ByAssignee   []AssigneeStatRow    `json:"by_assignee"`
}

type AssigneeStatRow struct {
	AssigneeID *string `json:"assignee_id"`
	Name       *string `json:"name"`
	Count      int     `json:"count"`
}
