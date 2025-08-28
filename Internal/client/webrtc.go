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

type DataChannelMessageHandler func(webrtc.DataChannelMessage, *WebRTCPeer)

type WebRTCPeer struct {
	pc              *webrtc.PeerConnection
	dataChannel     *webrtc.DataChannel
	onMessage       DataChannelMessageHandler
	fileWriter      io.WriteCloser
	state           webrtc.PeerConnectionState
	connectedSignal chan struct{}
	mu              sync.RWMutex
	signalingStream network.Stream
}

func NewWebRTCPeer(onMessage DataChannelMessageHandler) (*WebRTCPeer, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.cloudflare.com:3478"}},
			{
				URLs: []string{
					"turn:openrelay.metered.ca:80",
					"turn:openrelay.metered.ca:443",
					"turns:openrelay.metered.ca:443",
				},
				Username:   "openrelayproject",
				Credential: "openrelayproject",
			},
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	peer := &WebRTCPeer{
		pc:              pc,
		onMessage:       onMessage,
		connectedSignal: make(chan struct{}),
	}

	pc.OnConnectionStateChange(peer.handleConnectionStateChange)
	pc.OnDataChannel(peer.handleDataChannel)

	return peer, nil
}

func (p *WebRTCPeer) handleConnectionStateChange(s webrtc.PeerConnectionState) {
	p.mu.Lock()
	p.state = s
	p.mu.Unlock()

	log.Printf("Peer Connection State changed: %s", s.String())

	if s == webrtc.PeerConnectionStateConnected {
		select {
		case <-p.connectedSignal:
		default:
			close(p.connectedSignal)
		}
	} else if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
		p.Close()
	}
}

func (p *WebRTCPeer) handleDataChannel(dc *webrtc.DataChannel) {
	log.Printf("New DataChannel %q (ID: %d) received!", dc.Label(), dc.ID())
	
	p.mu.Lock()
	p.dataChannel = dc
	p.mu.Unlock()

	dc.OnOpen(func() {
		log.Printf("Data channel '%s' (ID: %d) opened!", dc.Label(), dc.ID())
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

func (p *WebRTCPeer) CreateOffer() (string, error) {
	dc, err := p.pc.CreateDataChannel("data", nil)
	if err != nil {
		return "", err
	}

	p.handleDataChannel(dc)

	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		return "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(p.pc)
	if err = p.pc.SetLocalDescription(offer); err != nil {
		return "", err
	}

	<-gatherComplete

	offerJSON, err := json.Marshal(p.pc.LocalDescription())
	return string(offerJSON), err
}

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

func (p *WebRTCPeer) SetAnswer(answerSDP string) error {
	var answer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(answerSDP), &answer); err != nil {
		return err
	}
	return p.pc.SetRemoteDescription(answer)
}

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

func (p *WebRTCPeer) SendRaw(data []byte) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.IsConnected() {
		return fmt.Errorf("data channel not open")
	}

	return p.dataChannel.Send(data)
}

func (p *WebRTCPeer) SetFileWriter(writer io.WriteCloser) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fileWriter = writer
}

func (p *WebRTCPeer) GetFileWriter() io.WriteCloser {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.fileWriter
}