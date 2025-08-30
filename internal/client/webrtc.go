package webRTC

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/pion/webrtc/v3"
)

// data channel pe aane wale messages ko handle karta hai
type DataChannelMessageHandler func(webrtc.DataChannelMessage, *WebRTCPeer)

// yeh struct ek webRTC connection aur related state ko show karta hai
type WebRTCPeer struct {
	pc              *webrtc.PeerConnection
	dataChannel     *webrtc.DataChannel
	onMessage       DataChannelMessageHandler
	fileWriter      io.WriteCloser
	state           webrtc.PeerConnectionState
	connectedSignal chan struct{} // Jab connection successfully ban jata hai to yeh channel close ho jata hai
	mu              sync.RWMutex  //concurrent access se protect karne ke liye
	signalingStream network.Stream
}

// ek naya webRTC peer bnata hai
func NewWebRTCPeer(onMessage DataChannelMessageHandler) (*WebRTCPeer, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			// Cloudflare STUN (free)
			{URLs: []string{"stun:stun.cloudflare.com:3478"}},

			// Metered.ca Open Relay (free 20GB/month, runs on ports 80/443)
			{
				URLs: []string{
					"turn:openrelay.metered.ca:80",
					"turn:openrelay.metered.ca:443",
					"turns:openrelay.metered.ca:443",
				},
				Username:   "openrelayproject",
				Credential: "openrelayproject",
			},

			// Backup STUN servers
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	// Naya peer connection banate hain.
	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	peer := &WebRTCPeer{
		pc:              pc,
		onMessage:       onMessage,
		connectedSignal: make(chan struct{}),
	}

	//this handles change in connection states
	pc.OnConnectionStateChange(peer.handleConnectionStateChange)
	// Jab remote peer ek data channel kholta hai
	pc.OnDataChannel(peer.handleDataChannel)

	return peer, nil
}

func (p *WebRTCPeer) handleConnectionStateChange(s webrtc.PeerConnectionState) {
	p.mu.Lock()
	p.state = s // this line updates the state change
	p.mu.Unlock()

	log.Printf("Peer Connection State has changed: %s\n", s.String())

	if s == webrtc.PeerConnectionStateConnected {
		//Jab connection ban jata hai, `connectedSignal` channel ko close karte hain
		// Yeh `WaitForConnection` mein waiting goroutine ko signal dega
		select {
		case <-p.connectedSignal:
		default:
			close(p.connectedSignal)
		}
	} else if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
		p.Close()
	}
}

// handleDataChannel tab call hota hai jab remote peer ek data channel banata hai.
func (p *WebRTCPeer) handleDataChannel(dc *webrtc.DataChannel) {
	log.Printf("New DataChannel %q (ID: %d) received!", dc.Label(), dc.ID())
	p.mu.Lock()
	p.dataChannel = dc
	p.mu.Unlock()

	dc.OnOpen(func() {
		log.Printf("Data channel '%s' (ID: %d) opened!", dc.Label(), dc.ID())
		// The connection is now fully established
		p.handleConnectionStateChange(webrtc.PeerConnectionStateConnected)
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		p.onMessage(msg, p)
	})
	dc.OnClose(func() {
		log.Printf("Data channel '%s' (ID: %d) closed!", dc.Label(), dc.ID())
		p.handleConnectionStateChange(webrtc.PeerConnectionStateClosed)
	})
}

// CreateOffer ek offer SDP generate karta hai
func (p *WebRTCPeer) CreateOffer() (string, error) {
	dc, err := p.pc.CreateDataChannel("data", nil)
	if err != nil {
		return "", err
	}
	p.handleDataChannel(dc)

	// Offer create karte hain
	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		return "", err
	}

	//ICE gathering ka wait
	gatherComplete := webrtc.GatheringCompletePromise(p.pc)
	if err = p.pc.SetLocalDescription(offer); err != nil {
		return "", err
	}
	<-gatherComplete // yeh wait end ka signal karta hai

	offerJSON, err := json.Marshal(p.pc.LocalDescription())
	return string(offerJSON), err
}

// CreateAnswer ek answer SDP generate karta hai.
func (p *WebRTCPeer) CreateAnswer(offerSDP string) (string, error) {
	var offer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(offerSDP), &offer); err != nil {
		return "", err
	}

	if err := p.pc.SetRemoteDescription(offer); err != nil {
		return "", err
	}

	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(p.pc)
	if err = p.pc.SetLocalDescription(answer); err != nil {
		return "", err
	}
	<-gatherComplete

	answerJSON, err := json.Marshal(p.pc.LocalDescription())
	return string(answerJSON), err
}

// SetAnswer remote peer se mile answer ko set karta hai.
func (p *WebRTCPeer) SetAnswer(answerSDP string) error {
	var answer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(answerSDP), &answer); err != nil {
		return err
	}
	return p.pc.SetRemoteDescription(answer)
}

// yeh function, specific timeout tak connection establish hone ka wait karta hai
func (p *WebRTCPeer) WaitForConnection(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-p.connectedSignal:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for connection: %w", ctx.Err())
	}
}

// peer ka connection state check
func (p *WebRTCPeer) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state == webrtc.PeerConnectionStateConnected
}

func (p *WebRTCPeer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.signalingStream != nil {
		p.signalingStream.Close()
		p.signalingStream = nil
	}

	if p.pc != nil && p.state != webrtc.PeerConnectionStateClosed {
		return p.pc.Close()
	}
	return nil
}

func (p *WebRTCPeer) SetSignalingStream(s network.Stream) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.signalingStream = s
}

// data ko JSON mein serialize kare data channels pe bhjeta hai
func (p *WebRTCPeer) Send(data interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.IsConnected() {
		return fmt.Errorf("data channel not open")
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return p.dataChannel.SendText(string(bytes))
}

// file data ko bytes mein data channel par bhejta hai.
func (p *WebRTCPeer) SendRaw(data []byte) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.IsConnected() {
		return fmt.Errorf("data channel not open")
	}
	return p.dataChannel.Send(data)
}

// Creates an empty file on your computer  (only called once)
// SetFileWriter(emptyFile) to attach this empty file to the connection
func (p *WebRTCPeer) SetFileWriter(writer io.WriteCloser) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fileWriter = writer
}

// retrieve the attached file
// gets called each time a chunk is sent
func (p *WebRTCPeer) GetFileWriter() io.WriteCloser {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.fileWriter
}
