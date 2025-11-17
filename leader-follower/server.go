package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// Server represents the HTTP server
type Server struct {
	port     string
	leader   *LeaderNode
	follower *FollowerNode
	nodeType string
}

// NewServer creates a new server
func NewServer(port string, leader *LeaderNode, follower *FollowerNode, nodeType string) *Server {
	return &Server{
		port:     port,
		leader:   leader,
		follower: follower,
		nodeType: nodeType,
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

// handleSet handles set requests : Leader - Follower write logic
func (s *Server) handleSet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if s.nodeType == "leader" {
		// Write to leader and replicate to followers
		statusCode, version, err := s.leader.Write(req.Key, req.Value)
		if err != nil {
			http.Error(w, err.Error(), statusCode)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"version": version,
		})
	} else {
		http.Error(w, "Write requests must go to leader", http.StatusForbidden)
	}
}

// handleGet handles get requests : Leader - Follower read logic
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	var statusCode int
	var value string
	var version int
	var err error

	// Leader serves read requests directly
	if s.nodeType == "leader" {
		statusCode, value, version, err = s.leader.Read(key)
	} else {
		// Follower serves read requests locally
		statusCode, value, version, err = s.follower.LocalRead(key)
	}

	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"value":   value,
		"version": version,
	})
}

// handleReplicate handles replication requests (follower only)
func (s *Server) handleReplicate(w http.ResponseWriter, r *http.Request) {
	if s.nodeType != "follower" {
		http.Error(w, "Not a follower", http.StatusForbidden)
		return
	}

	var req struct {
		Key     string `json:"key"`
		Value   string `json:"value"`
		Version int    `json:"version"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Follower handles replication requests
	statusCode := s.follower.Replicate(req.Key, req.Value, req.Version)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "replicated",
	})
}

// handleLocalRead handles local read requests (for testing)
func (s *Server) handleLocalRead(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	var statusCode int
	var value string
	var version int
	var err error

	if s.nodeType == "leader" {
		statusCode, value, version, err = s.leader.LocalRead(key)
	} else {
		statusCode, value, version, err = s.follower.LocalRead(key)
	}

	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"value":   value,
		"version": version,
	})
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "healthy",
		"node_type": s.nodeType,
	})
}
