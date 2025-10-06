package p2p

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

const SignalingProtocolID = "/torrentium/webrtc-signaling/1.0"

type SignalingMessage struct {
	Type string `json:"type"` // "offer", "answer", "ice-candidate", "close"
	Data string `json:"data"`
}

// RegisterSignalingProtocol sets up WebRTC signaling protocol handler
func RegisterSignalingProtocol(h host.Host, onOffer func(offer, remotePeerID string, s network.Stream) (string, error)) {
	h.SetStreamHandler(SignalingProtocolID, func(s network.Stream) {
		log.Printf("Received incoming signaling connection from %s", s.Conn().RemotePeer())

		decoder := json.NewDecoder(s)
		encoder := json.NewEncoder(s)

		var msg SignalingMessage
		if err := decoder.Decode(&msg); err != nil {
			log.Printf("Error decoding signaling message: %v", err)
			_ = s.Reset()
			return
		}

		if msg.Type != "offer" {
			log.Printf("Expected offer, got: %s", msg.Type)
			_ = s.Reset()
			return
		}

		answer, err := onOffer(msg.Data, s.Conn().RemotePeer().String(), s)
		if err != nil {
			log.Printf("Error handling offer: %v", err)
			errorMsg := SignalingMessage{
				Type: "error",
				Data: fmt.Sprintf("ERROR:%s", err.Error()),
			}
			_ = encoder.Encode(errorMsg)
			_ = s.Reset()
			return
		}

		answerMsg := SignalingMessage{
			Type: "answer",
			Data: answer,
		}

		if err := encoder.Encode(answerMsg); err != nil {
			log.Printf("Error encoding answer: %v", err)
			_ = s.Reset()
			return
		}

		// Keep stream open for ICE candidate exchange
		log.Printf("Signaling stream established with %s", s.Conn().RemotePeer())

		// Handle additional signaling messages (ICE candidates, etc.)
		for {
			var additionalMsg SignalingMessage
			if err := decoder.Decode(&additionalMsg); err != nil {
				log.Printf("Signaling stream closed or error: %v", err)
				break
			}

			switch additionalMsg.Type {
			case "close":
				log.Printf("Peer requested signaling stream close")
				return
			case "ice-candidate":
				log.Printf("Received ICE candidate from peer")
			default:
				log.Printf("Unknown signaling message type: %s", additionalMsg.Type)
			}
		}
	})
}
