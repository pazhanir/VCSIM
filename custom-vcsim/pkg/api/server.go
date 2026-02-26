// Package api provides the REST API for the scenario controller.
// This runs on a separate port (default :8990) from the vSphere API (443).
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/site24x7/vcsim-demo/pkg/scenarios"
)

// Server is the REST API server for scenario management.
type Server struct {
	manager *scenarios.Manager
	addr    string
}

// NewServer creates a new API server.
func NewServer(manager *scenarios.Manager, addr string) *Server {
	return &Server{
		manager: manager,
		addr:    addr,
	}
}

// Start starts the API server (blocking).
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/scenarios", s.handleListScenarios)
	mux.HandleFunc("/api/scenarios/active", s.handleListActive)
	mux.HandleFunc("/api/scenario/activate", s.handleActivate)
	mux.HandleFunc("/api/scenario/deactivate", s.handleDeactivate)
	mux.HandleFunc("/api/scenario/clear-all", s.handleClearAll)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/", s.handleIndex)

	log.Printf("[api] Scenario controller listening on %s", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "vcsim-scenario-controller",
		"version": "1.0.0",
		"endpoints": []string{
			"GET  /api/scenarios        - List all available scenarios",
			"GET  /api/scenarios/active - List currently active scenarios",
			"POST /api/scenario/activate    - Activate a scenario",
			"POST /api/scenario/deactivate  - Deactivate a scenario",
			"POST /api/scenario/clear-all   - Clear all active scenarios",
			"GET  /api/health               - Health check",
		},
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleListScenarios(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	defs := s.manager.ListDefinitions()

	// Group by category
	grouped := make(map[string][]scenarios.ScenarioDef)
	for _, d := range defs {
		cat := string(d.Category)
		grouped[cat] = append(grouped[cat], d)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":      len(defs),
		"categories": grouped,
	})
}

func (s *Server) handleListActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	active := s.manager.ListActive()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":  len(active),
		"active": active,
	})
}

func (s *Server) handleActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req scenarios.ActivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: %v", err)
		return
	}

	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "Missing 'id' field")
		return
	}
	if len(req.Targets) == 0 {
		writeError(w, http.StatusBadRequest, "Missing 'targets' field")
		return
	}

	ctx := context.Background()
	if err := s.manager.Activate(ctx, req); err != nil {
		writeError(w, http.StatusInternalServerError, "Activation failed: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "activated",
		"id":      req.ID,
		"targets": req.Targets,
	})
}

func (s *Server) handleDeactivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req scenarios.ActivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: %v", err)
		return
	}

	ctx := context.Background()
	if err := s.manager.Deactivate(ctx, req); err != nil {
		writeError(w, http.StatusInternalServerError, "Deactivation failed: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "deactivated",
		"id":      req.ID,
		"targets": req.Targets,
	})
}

func (s *Server) handleClearAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	if err := s.manager.ClearAll(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "Clear failed: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

func writeError(w http.ResponseWriter, code int, format string, args ...interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": fmt.Sprintf(format, args...),
	})
}
