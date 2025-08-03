package main

import (
	"context"
	"log"
	"os"

	"torrentium/db"
	"torrentium/p2p"
	"torrentium/tracker"

	"github.com/joho/godotenv"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

//this is thhe  entry point of the tracker related code
func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Unable to access .env file")
	}


	//db connection initialize kara
	db.InitDB()

	// On startup, saare peers ko offline mark kar rhe hai(emulating a server restart)
	repo := db.NewRepository(db.DB)
	if err := repo.MarkAllPeersOffline(context.Background()); err != nil {
		log.Printf("Warning: Could not mark all peers offline on startup: %v", err)
	}
	log.Println("-> Cleared stale online peer statuses.")

	// .env se tracker ka listening address fetch kar rhe hai
	listenAddr := os.Getenv("TRACKER_LISTEN_ADDR")
	if listenAddr == "" {
		log.Fatal("TRACKER_LISTEN_ADDR environment variable not set.")
	}

	//ek P2P host bnate hai jo connections ke liye listen karega
	h, err := p2p.NewHost(context.Background(), listenAddr)
	if err != nil {
		log.Fatal(err)
	}

	_, err = relay.New(h, relay.WithResources(relay.DefaultResources()))
	if err != nil {
		log.Fatalf("Failed to initiate relay: %v", err)
	}

	log.Println("-> Tracker Initialized as a Relay")

	// Naya tracker instance banate hai
	t := tracker.NewTracker()
	log.Println("-> Tracker Initialized")

	
	//yeh host aur tracker ko link/connect karta hai
	//host (h) - knows how to receive connections, but it doesn't know what to do with them once they're established
	//tracker (t) - brains of the operation. knows what to do but has no way to directly receive requests from the network
	p2p.RegisterTrackerProtocol(h, t)

	select {}
}