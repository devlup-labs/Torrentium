package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"torrentium/tracker"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

const TrackerProtocolID = "/torrentium/tracker/1.0"

type Message struct {
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type HandshakePayload struct {
	Name        string   `json:"name"`
	ListenAddrs []string `json:"listen_addrs"`
}

type AnnounceFilePayload struct {
	FileHash string `json:"file_hash"`
	Filename string `json:"filename"`
	FileSize int64  `json:"file_size"`
}

type GetPeersPayload struct {
	FileID uuid.UUID `json:"file_id"`
}

type GetPeerInfoPayload struct {
	PeerDBID uuid.UUID `json:"peer_db_id"`
}

type AnnounceAckPayload struct {
	FileID uuid.UUID `json:"file_id"`
}

func RegisterTrackerProtocol(h host.Host, t *tracker.Tracker) {
	h.SetStreamHandler(TrackerProtocolID, func(s network.Stream) {
		log.Printf("New peer connected: %s", s.Conn().RemotePeer())
		ctx := context.Background()
		defer s.Close()
		defer t.RemovePeer(ctx, s.Conn().RemotePeer().String())

		if err := handleStream(ctx, s, t); err != nil {
			if err != io.EOF {
				log.Printf("Error handling stream for peer %s: %v", s.Conn().RemotePeer(), err)
			}
		}
	})
}

func handleStream(ctx context.Context, s network.Stream, t *tracker.Tracker) error {
	decoder := json.NewDecoder(s)
	encoder := json.NewEncoder(s)
	remotePeerID := s.Conn().RemotePeer().String()

	var nameMsg Message
	if err := decoder.Decode(&nameMsg); err != nil {
		return err
	}
	if nameMsg.Command != "HANDSHAKE" {
		return encoder.Encode(Message{Command: "ERROR", Payload: json.RawMessage(`"Expected HANDSHAKE command"`)})
	}

	var handshake HandshakePayload
	if err := json.Unmarshal(nameMsg.Payload, &handshake); err != nil || handshake.Name == "" {
		return encoder.Encode(Message{Command: "ERROR", Payload: json.RawMessage(`"Invalid handshake payload"`)})
	}

	if err := t.AddPeer(ctx, remotePeerID, handshake.Name, handshake.ListenAddrs); err != nil {
		log.Printf("Failed to AddPeer to database: %v", err)
		return encoder.Encode(Message{Command: "ERROR", Payload: json.RawMessage(`"Failed to register with tracker"`)})
	}

	welcomePayload, _ := json.Marshal(t.ListPeers())
	encoder.Encode(Message{Command: "Tracker Connection Established", Payload: welcomePayload})

	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			return err
		}

		log.Printf("Received command '%s' from peer %s", msg.Command, remotePeerID)

		var response Message
		switch msg.Command {
		case "ANNOUNCE_FILE":
			var p AnnounceFilePayload
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				response.Command = "ERROR"
				response.Payload = json.RawMessage(fmt.Sprintf(`"Unmarshal error: %s"`, err.Error()))
			} else {
				fileID, err := t.AddFileWithPeer(ctx, p.FileHash, p.Filename, p.FileSize, remotePeerID)
				if err != nil {
					response.Command = "ERROR"
					response.Payload = json.RawMessage(fmt.Sprintf(`"Database error: %s"`, err.Error()))
				} else {
					response.Command = "ACK"
					ackPayload := AnnounceAckPayload{FileID: fileID}
					response.Payload, _ = json.Marshal(ackPayload)
				}
			}
		case "LIST_FILES":
			files, err := t.GetAllFiles(ctx)
			if err != nil {
				response.Command = "ERROR"
			} else {
				response.Command = "FILE_LIST"
				response.Payload, _ = json.Marshal(files)
			}
		case "GET_PEERS_FOR_FILE":
			var p GetPeersPayload
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				response.Command = "ERROR"
			} else {
				peers, err := t.GetOnlinePeersForFile(ctx, p.FileID)
				if err != nil {
					response.Command = "ERROR"
				} else {
					response.Command = "PEER_LIST"
					response.Payload, _ = json.Marshal(peers)
				}
			}
		case "GET_PEER_INFO":
			var p GetPeerInfoPayload
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				response.Command = "ERROR"
			} else {
				peerInfo, err := t.GetPeerInfoByDBID(ctx, p.PeerDBID)
				if err != nil {
					response.Command = "ERROR"
				} else {
					response.Command = "PEER_INFO"
					response.Payload, _ = json.Marshal(peerInfo)
				}
			}
		case "LIST_PEERS":
			peers, err := t.GetOnlinePeers(ctx)
			if err != nil {
				response.Command = "ERROR"
			} else {
				response.Command = "PEER_LIST_ALL"
				response.Payload, _ = json.Marshal(peers)
			}
		default:
			response.Command = "ERROR"
			response.Payload, _ = json.Marshal("Unknown command")
		}

		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
}