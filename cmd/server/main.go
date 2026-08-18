package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/cosmin-harangus/go-bpmn-engine/engine"
	pgstore "github.com/cosmin-harangus/go-bpmn-engine/store/pg"
	"github.com/cosmin-harangus/go-bpmn-engine/timer"
	bpmnserver "github.com/cosmin-harangus/go-bpmn-server/server"
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

func (a *engineAdapter) CompleteJob(ctx context.Context, jobID string, vars map[string]any) error {
	return a.eng.CompleteJob(ctx, jobID, vars)
}

func (a *engineAdapter) FailJob(ctx context.Context, jobID string, retries int, errMsg string) error {
	return a.eng.FailJob(ctx, jobID, retries, errMsg)
}

func (a *engineAdapter) CompleteUserTask(ctx context.Context, taskID string, vars map[string]any) error {
	return a.eng.CompleteUserTask(ctx, taskID, vars)
}

func (a *engineAdapter) PublishMessage(ctx context.Context, messageName, correlationKey string, vars map[string]any) error {
	return a.eng.PublishMessage(ctx, messageName, correlationKey, vars)
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

	srv := bpmnserver.New(&engineAdapter{eng: eng}, ":"+addr)
	slog.Info("starting server", "addr", addr)
	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
