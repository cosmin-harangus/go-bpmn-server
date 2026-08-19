package server

import (
	"context"
	"net/http"

	"github.com/cosmin-harangus/go-bpmn-engine/store"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	engine Engine
	addr   string
	srv    *http.Server
}

// Engine is the subset of engine.Engine used by the server.
type Engine interface {
	// Process deployment
	DeployProcess(ctx context.Context, bpmnXML []byte) (interface{}, error)

	// Instance lifecycle
	CreateInstance(ctx context.Context, processKey string, vars map[string]any) (string, error)
	RunInstance(ctx context.Context, instanceID string) error
	ListInstances(ctx context.Context, query store.InstanceQuery) ([]store.ProcessInstance, int, error)
	GetInstanceDetail(ctx context.Context, instanceID string) (*store.InstanceDetail, error)
	CancelInstance(ctx context.Context, instanceID string) error
	SuspendInstance(ctx context.Context, instanceID string) error
	ResumeInstance(ctx context.Context, instanceID string) error

	// Jobs
	CompleteJob(ctx context.Context, jobID string, vars map[string]any) error
	FailJob(ctx context.Context, jobID string, retries int, errMsg string) error

	// User tasks
	CompleteUserTask(ctx context.Context, taskID string, vars map[string]any) error
	ListUserTasks(ctx context.Context, query store.UserTaskQuery) ([]store.UserTask, int, error)
	ClaimTask(ctx context.Context, taskID, userID string) error
	UnclaimTask(ctx context.Context, taskID string) error

	// Incidents
	ListIncidents(ctx context.Context, query store.IncidentQuery) ([]store.Incident, int, error)
	ResolveIncident(ctx context.Context, incidentID string, retries int, vars map[string]any) error

	// Messages
	PublishMessage(ctx context.Context, messageName, correlationKey string, vars map[string]any) error
}

func New(e Engine, addr string) *Server {
	s := &Server{engine: e, addr: addr}
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(TenantFromHeader)

	// Processes
	r.Post("/processes", s.handleDeployProcess)

	// Instances
	r.Get("/instances", s.handleListInstances)
	r.Post("/instances", s.handleCreateInstance)
	r.Get("/instances/{id}", s.handleGetInstance)
	r.Post("/instances/{id}/run", s.handleRunInstance)
	r.Post("/instances/{id}/cancel", s.handleCancelInstance)
	r.Post("/instances/{id}/suspend", s.handleSuspendInstance)
	r.Post("/instances/{id}/resume", s.handleResumeInstance)

	// Jobs
	r.Post("/jobs/{id}/complete", s.handleCompleteJob)
	r.Post("/jobs/{id}/fail", s.handleFailJob)

	// User tasks
	r.Get("/user-tasks", s.handleListUserTasks)
	r.Post("/user-tasks/{id}/complete", s.handleCompleteUserTask)
	r.Post("/user-tasks/{id}/claim", s.handleClaimTask)
	r.Post("/user-tasks/{id}/unclaim", s.handleUnclaimTask)

	// Incidents
	r.Get("/incidents", s.handleListIncidents)
	r.Post("/incidents/{id}/resolve", s.handleResolveIncident)

	// Messages
	r.Post("/messages", s.handlePublishMessage)

	s.srv = &http.Server{Addr: addr, Handler: r}
	return s
}

// Handler returns the HTTP handler for testing purposes.
func (s *Server) Handler() http.Handler { return s.srv.Handler }

func (s *Server) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.srv.Shutdown(context.Background())
	}()
	if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
