package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// Server represents the HTTP server
type Server struct {
	port string
	node *LeaderlessNode
}

// NewServer creates a new server
func NewServer(port string, node *LeaderlessNode) *Server {
	return &Server{
		port: port,
		node: node,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	r := mux.NewRouter()

	// Register routes
	r.HandleFunc("/set", s.handleSet).Methods("POST")
	r.HandleFunc("/get/{key}", s.handleGet).Methods("GET")
	r.HandleFunc("/replicate", s.handleReplicate).Methods("POST")
	r.HandleFunc("/local_read/{key}", s.handleLocalRead).Methods("GET")
	r.HandleFunc("/health", s.handleHealth).Methods("GET")

	return http.ListenAndServe(":"+s.port, r)
}

// handleSet handles set requests - this node becomes write coordinator
func (s *Server) handleSet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	statusCode, version, err := s.node.Write(req.Key, req.Value)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":     version,
		"coordinator": s.node.nodeID,
	})
}

// handleGet handles get requests (R=1: read from local)
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	statusCode, value, version, err := s.node.Read(key)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"value":   value,
		"version": version,
		"node":    s.node.nodeID,
	})
}

// handleReplicate handles replication requests from other nodes
func (s *Server) handleReplicate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key     string `json:"key"`
		Value   string `json:"value"`
		Version int    `json:"version"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	statusCode := s.node.Replicate(req.Key, req.Value, req.Version)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "replicated",
		"node":   s.node.nodeID,
	})
}

// handleLocalRead handles local read requests (for testing)
func (s *Server) handleLocalRead(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	statusCode, value, version, err := s.node.LocalRead(key)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"value":   value,
		"version": version,
		"node":    s.node.nodeID,
	})
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"node_id": s.node.nodeID,
	})
}
