package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	awmv1 "github.com/enriquepascalin/awm-orchestrator/internal/proto/awm/v1"
)

//go:embed static/*
var staticFiles embed.FS

// Server provides a local web dashboard for human agents to view and manage tasks.
type Server struct {
	httpServer *http.Server
	port       int

	mu         sync.RWMutex
	tasks      map[string]*TaskInfo
	taskQueue  []*TaskInfo
	currentIdx int
}

// TaskInfo holds dashboard‑visible information about a task.
type TaskInfo struct {
	ID           string                 `json:"id"`
	ActivityName string                 `json:"activity_name"`
	Input        map[string]interface{} `json:"input"`
	Status       string                 `json:"status"` // pending, running, completed, failed
	Result       map[string]interface{} `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// NewServer creates a new dashboard server on the given port.
func NewServer(port int) *Server {
	return &Server{
		port:  port,
		tasks: make(map[string]*TaskInfo),
	}
}

// Start begins listening for HTTP requests. It blocks until the server is stopped.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/tasks", s.handleTasksList)
	mux.HandleFunc("/api/tasks/next", s.handleNextTask)
	mux.HandleFunc("/api/tasks/{id}", s.handleTaskByID)
	mux.HandleFunc("/api/tasks/{id}/submit", s.handleTaskSubmit)

	// Static file serving (embedded React/Vue app)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("failed to load embedded static files: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("Dashboard listening on http://localhost:%d", s.port)
	return s.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the dashboard server.
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// AddTask adds a new task to the dashboard queue.
func (s *Server) AddTask(task *awmv1.TaskAssignment) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info := &TaskInfo{
		ID:           task.TaskId,
		ActivityName: task.ActivityName,
		Input:        task.Input.AsMap(),
		Status:       "pending",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	s.tasks[task.TaskId] = info
	s.taskQueue = append(s.taskQueue, info)
}

// UpdateTaskStatus updates the status of a task.
func (s *Server) UpdateTaskStatus(taskID, status string, result map[string]interface{}, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if info, ok := s.tasks[taskID]; ok {
		info.Status = status
		info.Result = result
		info.Error = errMsg
		info.UpdatedAt = time.Now()
	}
}

// GetNextTask returns the next pending task (FIFO) or nil.
func (s *Server) GetNextTask() *TaskInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := 0; i < len(s.taskQueue); i++ {
		idx := (s.currentIdx + i) % len(s.taskQueue)
		if s.taskQueue[idx].Status == "pending" {
			s.currentIdx = (idx + 1) % len(s.taskQueue)
			return s.taskQueue[idx]
		}
	}
	return nil
}

// --- HTTP Handlers ---

func (s *Server) handleTasksList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*TaskInfo, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (s *Server) handleNextTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	task := s.GetNextTask()
	if task == nil {
		http.Error(w, "no pending tasks", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (s *Server) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	taskID := r.PathValue("id")
	s.mu.RLock()
	task, ok := s.tasks[taskID]
	s.mu.RUnlock()
	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (s *Server) handleTaskSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	taskID := r.PathValue("id")

	var submission struct {
		Result map[string]interface{} `json:"result"`
		Error  string                 `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&submission); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if ok && task.Status == "pending" {
		if submission.Error != "" {
			task.Status = "failed"
			task.Error = submission.Error
		} else {
			task.Status = "completed"
			task.Result = submission.Result
		}
		task.UpdatedAt = time.Now()
	}
	s.mu.Unlock()

	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}
