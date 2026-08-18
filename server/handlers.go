package server

import (
	"encoding/json"
	"io"
	"net/http"

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

func (s *Server) handleRunInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, 400, "invalid id")
		return
	}
	if err := s.engine.RunInstance(r.Context(), id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

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

func (s *Server) handleCompleteJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, 400, "invalid id")
		return
	}
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
	if id == "" {
		writeError(w, 400, "invalid id")
		return
	}
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

func (s *Server) handleCompleteUserTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, 400, "invalid id")
		return
	}
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
