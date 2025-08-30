package p2p

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	
)

//ek unique id jo WebRTC signaling ke mein use hogi.Peer A ko peer B ke beech transfer mein
const SignalingProtocolID = "/torrentium/webrtc-signaling/1.0"

// RegisterSignalingProtocol webRTC offer ke liye stream handler setup karta hai, jab koi peer protocolID pe join hota hai
func RegisterSignalingProtocol(h host.Host, onOffer func(offer, remotePeerID string, s network.Stream) (string, error)) {
	h.SetStreamHandler(SignalingProtocolID, func(s network.Stream) {
		log.Printf("Received incoming signaling connection from %s", s.Conn().RemotePeer())
		// defer s.Close()

		// JSON data ko stream se decode aur encode ke liye helpers
		decoder := json.NewDecoder(s)
		encoder := json.NewEncoder(s)

		var offer string
		if err := decoder.Decode(&offer); err != nil {
			log.Printf("Error decoding offer: %v", err)
			s.Reset()
			return
		}

		//yeh funcction offer ko proccess karke answer generate karta hai
		answer, err := onOffer(offer, s.Conn().RemotePeer().String(), s)
		if err != nil {
			log.Printf("Error handling offer: %v", err)
			encoder.Encode(fmt.Sprintf("ERROR:%s", err.Error()))
			s.Reset()
			return
		}

		//generated answer ko encode karke return kar dete hai
		if err := encoder.Encode(answer); err != nil {
			log.Printf("Error encoding answer: %v", err)
			s.Reset()
		}
	})
}