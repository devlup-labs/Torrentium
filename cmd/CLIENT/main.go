package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"torrentium/internal/cli"
	"torrentium/internal/core"
	db "torrentium/internal/db"
	p2p "torrentium/internal/p2p"

	"github.com/joho/godotenv"
	host "github.com/libp2p/go-libp2p/core/host"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := godotenv.Load(); err != nil {
		//log.Printf("Warning: Could not load .env file: %v", err)
	}

	DB := db.InitDB()
	if DB == nil {
		log.Fatal("Database initialization failed")
	}

	h, d, err := p2p.NewHost(
		ctx,
		"/ip4/0.0.0.0/tcp/0",
		nil, // temporarily, if you don’t have client yet
	)
	if err != nil {
		log.Fatal("Failed to create libp2p host:", err)
	}
	defer h.Close()

	go func() {
		if err := p2p.Bootstrap(ctx, h, d); err != nil {
			log.Printf("Error bootstrapping DHT: %v", err)
		}
	}()

	setupGracefulShutdown(h)

	repo := db.NewRepository(DB)
	client := core.NewClient(h, d, repo)
	client.StartDHTMaintenance()
	p2p.RegisterSignalingProtocol(h, client.HandleWebRTCOffer)

	cli.Execute(client)
}

func setupGracefulShutdown(h host.Host) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		log.Println("Shutting down gracefully...")
		_ = h.Close()
		os.Exit(0)
	}()
}
