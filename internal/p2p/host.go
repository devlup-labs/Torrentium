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
	ma "github.com/multiformats/go-multiaddr"
)

// privKeyFile constant hamari private key file ka naam store karta hai.
//iss private key se ek unique peerID attch ho jaegi, this solves our reusability of our node
const privKeyFile = "private_key"

// newHost func ek naya libp2p host bnata hai
// Yeh private key load ya generate karta hai aur provided address par listen karta hai.
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
		//private key se host ki identity in the network create ho rhi hai
		libp2p.Identity(priv),
		libp2p.ListenAddrs(maddr),
	)
	if err != nil {
		return nil, err
	}

	var actualAddrs []string
	for _, addr := range h.Addrs() {
		actualAddrs = append(actualAddrs, addr.String())
	}

	fmt.Printf("✅ Tracker host ID: %s\n", h.ID())
	fmt.Printf("✅ Tracker listening on: %s/p2p/%s\n", strings.Join(actualAddrs, ", "), h.ID())

	return h, nil
}

// loadOrGeneratePrivateKey function file se private key load karta hai.
// Agar private key already present nhi hai toh yeh ek new file generate karta hai
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

		//permissions 0600 - sirf owner hi read/write kar sakta hai
		if err := os.WriteFile(privKeyFile, privBytes, 0600); err != nil {
			return nil, fmt.Errorf("failed to write private key to file: %w", err)
		}
		log.Println("Generated new libp2p private key.")
		return priv, nil
	} else if err != nil {
		return nil, err // Other error, e.g., permissions
	}

	log.Println("Loaded existing libp2p private key.")
	return crypto.UnmarshalPrivateKey(privBytes)
}

//Basically Libp2p uses this private key to generate a unique Peer ID. 
// This ID is derived from the public key and serves as the node's permanent 
// and verifiable address on the network
