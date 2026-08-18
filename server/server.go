package server

import (
	"context"
	"net/http"

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
	DeployProcess(ctx context.Context, bpmnXML []byte) (interface{}, error)
	CreateInstance(ctx context.Context, processKey string, vars map[string]any) (string, error)
	RunInstance(ctx context.Context, instanceID string) error
	CompleteJob(ctx context.Context, jobID string, vars map[string]any) error
	FailJob(ctx context.Context, jobID string, retries int, errMsg string) error
	CompleteUserTask(ctx context.Context, taskID string, vars map[string]any) error
	PublishMessage(ctx context.Context, messageName, correlationKey string, vars map[string]any) error
}

func New(e Engine, addr string) *Server {
	s := &Server{engine: e, addr: addr}
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(TenantFromHeader)
	r.Post("/processes", s.handleDeployProcess)
	r.Post("/instances", s.handleCreateInstance)
	r.Post("/instances/{id}/run", s.handleRunInstance)
	r.Post("/messages", s.handlePublishMessage)
	r.Post("/jobs/{id}/complete", s.handleCompleteJob)
	r.Post("/jobs/{id}/fail", s.handleFailJob)
	r.Post("/user-tasks/{id}/complete", s.handleCompleteUserTask)
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
