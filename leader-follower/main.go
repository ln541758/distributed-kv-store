package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// createStore creates a Store based on BACKEND_TYPE environment variable
// Returns: memory store if BACKEND_TYPE=memory or not set
//
//	S3 store if BACKEND_TYPE=s3
func createStore() (Store, error) {
	backendType := os.Getenv("BACKEND_TYPE")
	if backendType == "" {
		backendType = "memory" // default to memory
	}

	log.Println("=== createStore() invoked ===")
	log.Println("BACKEND_TYPE =", backendType)
	log.Println("S3_BUCKET =", os.Getenv("S3_BUCKET"))
	log.Println("S3_ENDPOINT =", os.Getenv("S3_ENDPOINT"))

	switch backendType {
	case "s3":
		bucket := os.Getenv("S3_BUCKET")
		if bucket == "" {
			return nil, fmt.Errorf("S3_BUCKET missing when BACKEND_TYPE=s3")
		}
		store, err := NewS3Store(bucket)
		if err != nil {
			return nil, err
		}
		log.Printf("Using S3 backend (bucket: %s)", bucket)
		return store, nil

	case "memory":
		fallthrough
	default:
		log.Printf("Using in-memory backend")
		return NewKVStore(), nil
	}
}

func main() {
	nodeType := os.Getenv("NODE_TYPE") // "leader" or "follower"
	if nodeType == "" {
		nodeType = "follower"
	}

	store, err := createStore()
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	w, _ := strconv.Atoi(os.Getenv("W"))
	if w == 0 {
		w = 5
	}

	r, _ := strconv.Atoi(os.Getenv("R"))
	if r == 0 {
		r = 1
	}

	var server *Server

	if nodeType == "leader" {
		// Parse follower URLs from environment
		followerURLs := []string{}
		if urls := os.Getenv("FOLLOWER_URLS"); urls != "" {
			followerURLs = strings.Split(urls, ",")
		}

		leader := NewLeaderNode(store, followerURLs, w, r)
		server = NewServer(port, leader, nil, nodeType)
	} else {
		follower := NewFollowerNode(store)
		server = NewServer(port, nil, follower, nodeType)
	}

	log.Printf("Starting %s node on port %s (W=%d, R=%d)\n", nodeType, port, w, r)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
