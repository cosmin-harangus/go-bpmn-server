package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/cosmin-harangus/go-bpmn-engine/store"
	"github.com/go-chi/chi/v5"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func queryInt(r *http.Request, key string, def int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// --- Processes ---

func (s *Server) handleDeployProcess(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, 400, "read body: "+err.Error())
		return
	}
	def, err := s.engine.DeployProcess(r.Context(), body)
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	writeJSON(w, 201, def)
}

// --- Instances ---

func (s *Server) handleListInstances(w http.ResponseWriter, r *http.Request) {
	q := store.InstanceQuery{
		State:         r.URL.Query().Get("state"),
		ProcessDefKey: r.URL.Query().Get("process_key"),
		Limit:         queryInt(r, "limit", 50),
		Offset:        queryInt(r, "offset", 0),
	}
	instances, total, err := s.engine.ListInstances(r.Context(), q)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": instances, "total": total})
}

func (s *Server) handleCreateInstance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProcessKey string         `json:"process_key"`
		Variables  map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	id, err := s.engine.CreateInstance(r.Context(), req.ProcessKey, req.Variables)
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	if err := s.engine.RunInstance(r.Context(), id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}

func (s *Server) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := s.engine.GetInstanceDetail(r.Context(), id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if detail == nil {
		writeError(w, 404, "instance not found")
		return
	}
	writeJSON(w, 200, detail)
}

func (s *Server) handleRunInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.engine.RunInstance(r.Context(), id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleCancelInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.engine.CancelInstance(r.Context(), id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleSuspendInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.engine.SuspendInstance(r.Context(), id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleResumeInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.engine.ResumeInstance(r.Context(), id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

// --- Jobs ---

func (s *Server) handleCompleteJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Variables map[string]any `json:"variables"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := s.engine.CompleteJob(r.Context(), id, req.Variables); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleFailJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Retries int    `json:"retries"`
		Message string `json:"message"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := s.engine.FailJob(r.Context(), id, req.Retries, req.Message); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

// --- User tasks ---

func (s *Server) handleListUserTasks(w http.ResponseWriter, r *http.Request) {
	q := store.UserTaskQuery{
		State:          r.URL.Query().Get("state"),
		Assignee:       r.URL.Query().Get("assignee"),
		CandidateGroup: r.URL.Query().Get("candidate_group"),
		ProcessDefKey:  r.URL.Query().Get("process_key"),
		InstanceID:     r.URL.Query().Get("instance_id"),
		Limit:          queryInt(r, "limit", 50),
		Offset:         queryInt(r, "offset", 0),
	}
	tasks, total, err := s.engine.ListUserTasks(r.Context(), q)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": tasks, "total": total})
}

func (s *Server) handleCompleteUserTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Variables map[string]any `json:"variables"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := s.engine.CompleteUserTask(r.Context(), id, req.Variables); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleClaimTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeError(w, 400, "user_id is required")
		return
	}
	if err := s.engine.ClaimTask(r.Context(), id, req.UserID); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleUnclaimTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.engine.UnclaimTask(r.Context(), id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

// --- Incidents ---

func (s *Server) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	q := store.IncidentQuery{
		State:      r.URL.Query().Get("state"),
		InstanceID: r.URL.Query().Get("instance_id"),
		Limit:      queryInt(r, "limit", 50),
		Offset:     queryInt(r, "offset", 0),
	}
	incidents, total, err := s.engine.ListIncidents(r.Context(), q)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": incidents, "total": total})
}

func (s *Server) handleResolveIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Retries   int            `json:"retries"`
		Variables map[string]any `json:"variables"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := s.engine.ResolveIncident(r.Context(), id, req.Retries, req.Variables); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

// --- Messages ---

func (s *Server) handlePublishMessage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MessageName    string         `json:"message_name"`
		CorrelationKey string         `json:"correlation_key"`
		Variables      map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if err := s.engine.PublishMessage(r.Context(), req.MessageName, req.CorrelationKey, req.Variables); err != nil {
		writeError(w, 422, err.Error())
		return
	}
	w.WriteHeader(204)
}
