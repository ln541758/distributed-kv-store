package main

import (
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	nodeType := os.Getenv("NODE_TYPE") // "leader" or "follower"
	if nodeType == "" {
		nodeType = "follower"
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

		leader := NewLeaderNode(followerURLs, w, r)
		server = NewServer(port, leader, nil, nodeType)
	} else {
		follower := NewFollowerNode()
		server = NewServer(port, nil, follower, nodeType)
	}

	log.Printf("Starting %s node on port %s (W=%d, R=%d)\n", nodeType, port, w, r)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
