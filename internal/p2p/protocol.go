package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	tracker "torrentium/internal/tracker"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

// TrackerProtocolID ek unique string hai jo tracker ko network par pehchanta hai.
// Isse peers ko pata chalta hai ki woh sahi tracker se baat kar rahe hain.
const TrackerProtocolID = "/torrentium/tracker_funcs/1.0"

// yeh struct tracker aur peer ke beech ke messaging ko define kar rha hai
type Message struct {
	Command string          `json:"command"`           // name of command jaise : ADD_PEER
	Payload json.RawMessage `json:"payload,omitempty"` // according to command, payload mein data hai
}

// yeh struct peer ke tracker ya kisi au peer ke saath handshake ko define karta hai
type HandshakePayload struct {
	Name        string   `json:"name"`
	ListenAddrs []string `json:"listen_addrs"`
	PeerID      string   `json:"peer_id"`
}

// AnnounceFilePayload struct tab use hota hai jab peer announce karta hai tracker ko ki uske paas ek nayi file hai.
type AnnounceFilePayload struct {
	FileHash string `json:"file_hash"`
	Filename string `json:"filename"`
	FileSize int64  `json:"file_size"`
	PeerID   string `json:"peer_id"`
}

// AnnounceAckPayload struct tracker se peer ko file announce karne par acknowledgement bhejne ke liye use hota hai.
type AnnounceAckPayload struct {
	FileID uuid.UUID `json:"file_id"`
}

// GetPeersPayload struct tab use hota hai jab peer ek specific file ke liye dusre peers ki list mangta hai.
type GetPeersPayload struct {
	FileID uuid.UUID `json:"file_id"` // file ka DB id jiske liye peeers chahiye
}

// yeh struct tab use hota hai jab kisi specific peer ki info chahiye(like yeh file kiske paas hai)
type GetPeerInfoPayload struct {
	PeerDBID uuid.UUID `json:"peer_db_id"`
}

// RequestFilePayload struct file request ke liye use hota hai
type RequestFilePayload struct {
	FileID          uuid.UUID `json:"file_id"`
	RequesterPeerID string    `json:"requester_peer_id"`
}

// FileTransferPayload struct file chunks transfer ke liye use hota hai
type FileTransferPayload struct {
	FileID     uuid.UUID `json:"file_id"`
	ChunkIndex int       `json:"chunk_index"`
	ChunkData  []byte    `json:"chunk_data"`
	IsLast     bool      `json:"is_last"`
	TotalSize  int64     `json:"total_size,omitempty"`
	Filename   string    `json:"filename,omitempty"`
}

// ConnectToPeerPayload struct direct peer connection ke liye use hota hai
type ConnectToPeerPayload struct {
	PeerID string `json:"peer_id"`
	Port   int    `json:"port"` // WebSocket server port
}

// RegisterTrackerProtocol function host par ek stream handler set karta hai.
// Jab bhi koi peer TrackerProtocolID ka use karke connect karta hai, toh yeh handler trigger hota hai.
func RegisterTrackerProtocol(h host.Host, t *tracker.Tracker) {
	h.SetStreamHandler(TrackerProtocolID, func(s network.Stream) {
		log.Printf("New peer connected: %s", s.Conn().RemotePeer())
		ctx := context.Background()
		defer s.Close()
		defer t.RemovePeerWithContext(ctx, s.Conn().RemotePeer().String())

		if err := handleStream(ctx, s, t); err != nil {
			if err != io.EOF {
				log.Printf("Error handling stream for peer %s: %v", s.Conn().RemotePeer(), err)
			}
		}
	})
}

// yeh function kisi indivisual peer ki stream se aaye hue messages handle karta gau
func handleStream(ctx context.Context, s network.Stream, t *tracker.Tracker) error {
	decoder := json.NewDecoder(s)
	encoder := json.NewEncoder(s)
	remotePeerID := s.Conn().RemotePeer().String()

	// first, peer se ek HANDSHAKE expect karte hai
	var nameMsg Message
	if err := decoder.Decode(&nameMsg); err != nil {
		return err
	}
	if nameMsg.Command != "HANDSHAKE" {
		return encoder.Encode(Message{Command: "ERROR", Payload: json.RawMessage(`"Expected HANDSHAKE command"`)})
	}

	// Handshake payload ko parse karte hain
	var handshake HandshakePayload
	if err := json.Unmarshal(nameMsg.Payload, &handshake); err != nil || handshake.Name == "" {
		return encoder.Encode(Message{Command: "ERROR", Payload: json.RawMessage(`"Invalid handshake payload"`)})
	}

	//yeh peer ko tracker aur database dono mein store karta hai
	if err := t.AddPeerWithContext(ctx, remotePeerID, handshake.Name, handshake.ListenAddrs); err != nil {
		log.Printf("CRITIC: failed to AddPeer to database: %v", err)
		return encoder.Encode(Message{Command: "ERROR", Payload: json.RawMessage(`"Failed to register with tracker"`)})
	}

	// Handshake success hone par, welcome message bhejte hain jisme online peers ki list hoti hai.
	welcomePayload, _ := json.Marshal(t.ListPeers())
	encoder.Encode(Message{Command: "Tracker Connection Established", Payload: welcomePayload})

	//yeh loop peer se commands ka wait kaeta hai, aur accordingly react karta hai
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
				log.Printf("ERROR in ANNOUNCE_FILE (unmarshal): %v", err)
				response.Command = "ERROR"
				response.Payload = json.RawMessage(fmt.Sprintf(`"%s"`, err.Error()))
			} else {
				//announcedd filee ko database mein peer ke saath link karte hai
				fileID, err := t.AddFileWithPeer(ctx, p.FileHash, p.Filename, p.FileSize, remotePeerID)
				if err != nil {
					log.Printf("ERROR in ANNOUNCE_FILE (db): %v", err)
					response.Command = "ERROR"
					response.Payload = json.RawMessage(fmt.Sprintf(`"%s"`, err.Error()))
				} else {
					//ACK - ackowleddgment hai yha
					response.Command = "ACK"
					ackPayload := AnnounceAckPayload{FileID: fileID}
					response.Payload, _ = json.Marshal(ackPayload)
				}
			}

		//agar peer ne saari announced/available files ki list maangi hai
		case "LIST_FILES":
			files, err := t.GetAllFiles(ctx)
			if err != nil {
				log.Printf("ERROR in LIST_FILES (db): %v", err)
				response.Command = "ERROR"
			} else {
				response.Command = "FILE_LIST"
				response.Payload, _ = json.Marshal(files)
			}

		//peer ne ek specif file ke liye peers ki list maangi hai
		case "GET_PEERS_FOR_FILE":
			var p GetPeersPayload
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				log.Printf("ERROR in GET_PEERS_FOR_FILE (unmarshal): %v", err)
				response.Command = "ERROR"
			} else {
				peers, err := t.GetOnlinePeersForFile(ctx, p.FileID)
				if err != nil {
					log.Printf("ERROR in GET_PEERS_FOR_FILE (db): %v", err)
					response.Command = "ERROR"
				} else {
					response.Command = "PEER_LIST"
					response.Payload, _ = json.Marshal(peers)
				}
			}

		//peer ne kisi specific peer ki info maangi hai
		case "GET_PEER_INFO":
			var p GetPeerInfoPayload
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				log.Printf("ERROR in GET_PEER_INFO (unmarshal): %v", err)
				response.Command = "ERROR"
			} else {
				peerInfo, err := t.GetPeerInfoByDBID(ctx, p.PeerDBID)
				if err != nil {
					log.Printf("ERROR in GET_PEER_INFO (db): %v", err)
					response.Command = "ERROR"
				} else {
					response.Command = "PEER_INFO"
					response.Payload, _ = json.Marshal(peerInfo)
				}
			}

		//peer ne sabhi online peers ki list maangi hai
		case "LIST_PEERS":
			peers, err := t.GetOnlinePeers(ctx)
			if err != nil {
				log.Printf("ERROR in LIST_PEERS (db): %v", err)
				response.Command = "ERROR"
			} else {
				response.Command = "PEER_LIST_ALL"
				response.Payload, _ = json.Marshal(peers)
			}

		default:
			response.Command = "ERROR"
			response.Payload, _ = json.Marshal("Unknown command")
		}

		//response ko encode karke peer ko bhjete hai
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
}
