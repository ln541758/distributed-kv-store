package main

import (
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = "node1"
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

	// Parse peer URLs from environment
	peerURLs := []string{}
	if urls := os.Getenv("PEER_URLS"); urls != "" {
		peerURLs = strings.Split(urls, ",")
	}

	node := NewLeaderlessNode(nodeID, peerURLs, w, r)
	server := NewServer(port, node)

	log.Printf("Starting leaderless node %s on port %s (W=%d, R=%d)\n", nodeID, port, w, r)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
