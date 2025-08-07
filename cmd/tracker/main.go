package main

import (
	"context"
	"log"
	"os"

	"torrentium/db"
	"torrentium/p2p"
	"torrentium/tracker"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Unable to access .env file")
	}

	db.InitDB()

	repo := db.NewRepository(db.DB)
	if err := repo.MarkAllPeersOffline(context.Background()); err != nil {
		log.Printf("Warning: Could not mark all peers offline on startup: %v", err)
	}
	log.Println("-> Cleared stale online peer statuses.")

	listenAddr := os.Getenv("TRACKER_LISTEN_ADDR")
	if listenAddr == "" {
		log.Fatal("TRACKER_LISTEN_ADDR environment variable not set.")
	}

	h, err := p2p.NewHost(context.Background(), listenAddr)
	if err != nil {
		log.Fatal(err)
	}

	// log.Println("-> Tracker Initialized")

	t := tracker.NewTracker()
	log.Println("-> Tracker Initialized")

	p2p.RegisterTrackerProtocol(h, t)

	select {}
}