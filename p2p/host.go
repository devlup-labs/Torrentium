package p2p

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/transport/websocket"
	ma "github.com/multiformats/go-multiaddr"
)

const privKeyFile = "private_key"

func NewHost(ctx context.Context, listenAddr string) (host.Host, error) {
	priv, err := loadOrGeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to load/generate private key: %w", err)
	}

	maddr, err := ma.NewMultiaddr(listenAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse listen address '%s': %w", listenAddr, err)
	}

	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrs(maddr),
		libp2p.Transport(websocket.New),
	)
	if err != nil {
		return nil, err
	}

	var actualAddrs []string
	for _, addr := range h.Addrs() {
		actualAddrs = append(actualAddrs, addr.String())
	}

	fmt.Printf("Tracker host ID: %s\n", h.ID())
	fmt.Printf("Tracker listening on: %s/p2p/%s\n", strings.Join(actualAddrs, ", "), h.ID())

	return h, nil
}

func loadOrGeneratePrivateKey() (crypto.PrivKey, error) {
	privBytes, err := os.ReadFile(privKeyFile)
	if os.IsNotExist(err) {
		priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return nil, err
		}

		privBytes, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(privKeyFile, privBytes, 0600); err != nil {
			return nil, fmt.Errorf("failed to write private key to file: %w", err)
		}
		log.Println("Generated new libp2p private key.")
		return priv, nil
	} else if err != nil {
		return nil, err
	}

	log.Println("Loaded existing libp2p private key.")
	return crypto.UnmarshalPrivateKey(privBytes)
}