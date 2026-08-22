package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/cosmin-harangus/go-bpmn-engine/engine"
	"github.com/cosmin-harangus/go-bpmn-engine/store"
	pgstore "github.com/cosmin-harangus/go-bpmn-engine/store/pg"
	"github.com/cosmin-harangus/go-bpmn-engine/timer"
	bpmnserver "github.com/cosmin-harangus/go-bpmn-server/server"
	"github.com/cosmin-harangus/go-bpmn-server/server/auth"
	"github.com/cosmin-harangus/go-bpmn-server/server/auth/noauth"
	"github.com/cosmin-harangus/go-bpmn-server/server/auth/oidc"
)

// engineAdapter wraps *engine.Engine to satisfy bpmnserver.Engine.
// DeployProcess on *engine.Engine returns (*store.ProcessDef, error), but
// bpmnserver.Engine requires (interface{}, error).
type engineAdapter struct {
	eng *engine.Engine
}

func (a *engineAdapter) DeployProcess(ctx context.Context, bpmnXML []byte) (interface{}, error) {
	return a.eng.DeployProcess(ctx, bpmnXML)
}

func (a *engineAdapter) CreateInstance(ctx context.Context, processKey string, vars map[string]any) (string, error) {
	return a.eng.CreateInstance(ctx, processKey, vars)
}

func (a *engineAdapter) RunInstance(ctx context.Context, instanceID string) error {
	return a.eng.RunInstance(ctx, instanceID)
}

func (a *engineAdapter) ListInstances(ctx context.Context, query store.InstanceQuery) ([]store.ProcessInstance, int, error) {
	return a.eng.ListInstances(ctx, query)
}

func (a *engineAdapter) GetInstanceDetail(ctx context.Context, instanceID string) (*store.InstanceDetail, error) {
	return a.eng.GetInstanceDetail(ctx, instanceID)
}

func (a *engineAdapter) CancelInstance(ctx context.Context, instanceID string) error {
	return a.eng.CancelInstance(ctx, instanceID)
}

func (a *engineAdapter) SuspendInstance(ctx context.Context, instanceID string) error {
	return a.eng.SuspendInstance(ctx, instanceID)
}

func (a *engineAdapter) ResumeInstance(ctx context.Context, instanceID string) error {
	return a.eng.ResumeInstance(ctx, instanceID)
}

func (a *engineAdapter) CompleteJob(ctx context.Context, jobID string, vars map[string]any) error {
	return a.eng.CompleteJob(ctx, jobID, vars)
}

func (a *engineAdapter) FailJob(ctx context.Context, jobID string, retries int, errMsg string) error {
	return a.eng.FailJob(ctx, jobID, retries, errMsg)
}

func (a *engineAdapter) CompleteUserTask(ctx context.Context, taskID string, vars map[string]any) error {
	return a.eng.CompleteUserTask(ctx, taskID, vars)
}

func (a *engineAdapter) ListUserTasks(ctx context.Context, query store.UserTaskQuery) ([]store.UserTask, int, error) {
	return a.eng.ListUserTasks(ctx, query)
}

func (a *engineAdapter) ClaimTask(ctx context.Context, taskID, userID string) error {
	return a.eng.ClaimTask(ctx, taskID, userID)
}

func (a *engineAdapter) UnclaimTask(ctx context.Context, taskID string) error {
	return a.eng.UnclaimTask(ctx, taskID)
}

func (a *engineAdapter) ListIncidents(ctx context.Context, query store.IncidentQuery) ([]store.Incident, int, error) {
	return a.eng.ListIncidents(ctx, query)
}

func (a *engineAdapter) ResolveIncident(ctx context.Context, incidentID string, retries int, vars map[string]any) error {
	return a.eng.ResolveIncident(ctx, incidentID, retries, vars)
}

func (a *engineAdapter) PublishMessage(ctx context.Context, messageName, correlationKey string, vars map[string]any) error {
	return a.eng.PublishMessage(ctx, messageName, correlationKey, vars)
}

func buildAuthenticator() (auth.Authenticator, error) {
	switch os.Getenv("AUTH_MODE") {
	case "oidc":
		issuer := os.Getenv("OIDC_ISSUER_URL")
		if issuer == "" {
			return nil, fmt.Errorf("OIDC_ISSUER_URL is required when AUTH_MODE=oidc")
		}
		return oidc.New(oidc.Config{
			IssuerURL:   issuer,
			TenantClaim: envOr("AUTH_TENANT_CLAIM", "org_id"),
			UserClaim:   envOr("AUTH_USER_CLAIM", "sub"),
		}, nil)
	default:
		// "noauth" or empty — trust X-Tenant-ID header directly.
		// For production use, set AUTH_MODE=oidc or AUTH_MODE=builtin.
		slog.Warn("AUTH_MODE not set: running in no-auth mode — do not use in production")
		return &noauth.NoAuth{}, nil
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := pgstore.RunMigrations(ctx, connStr); err != nil {
		slog.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	s, err := pgstore.New(connStr)
	if err != nil {
		slog.Error("store init failed", "error", err)
		os.Exit(1)
	}

	eng, err := engine.New(s)
	if err != nil {
		slog.Error("engine init failed", "error", err)
		os.Exit(1)
	}

	timerSvc, err := timer.New(s, eng)
	if err != nil {
		slog.Error("timer init failed", "error", err)
		os.Exit(1)
	}

	go func() {
		if err := timerSvc.Start(ctx); err != nil && err != context.Canceled {
			slog.Error("timer service error", "error", err)
		}
	}()

	authenticator, err := buildAuthenticator()
	if err != nil {
		slog.Error("auth init failed", "error", err)
		os.Exit(1)
	}

	srv := bpmnserver.New(&engineAdapter{eng: eng}, ":"+addr, authenticator)
	slog.Info("starting server", "addr", addr)
	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
