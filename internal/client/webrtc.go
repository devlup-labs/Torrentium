package client

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pion/webrtc/v3"
)

const (
	maxICEGatheringTimeout = 15 * time.Second
	connectionTimeout      = 30 * time.Second
	keepAliveInterval      = 15 * time.Second
)

var webrtcConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{
				"stun:stun.l.google.com:19302",
				"stun:stun1.l.google.com:19302",
				"stun:stun.cloudflare.com:3478",
			},
		},
	},
}

type ConnectionState int

const (
	ConnectionStateNew ConnectionState = iota
	ConnectionStateConnecting
	ConnectionStateConnected
	ConnectionStateDisconnected
	ConnectionStateFailed
	ConnectionStateClosed
)

type SimpleWebRTCPeer struct {
	ID                peer.ID
	pc                *webrtc.PeerConnection
	dc                *webrtc.DataChannel
	reliableDC        *webrtc.DataChannel
	onMessage         func(msg webrtc.DataChannelMessage, peer *SimpleWebRTCPeer)
	onCloseCallback   func(peerID peer.ID)
	fileWriter        io.WriteCloser
	writerMutex       sync.RWMutex
	signalingStream   network.Stream
	streamMux         sync.RWMutex
	state             ConnectionState
	stateMux          sync.RWMutex
	closeOnce         sync.Once
	closeCh           chan struct{}
	keepAliveTick     *time.Ticker
	reliableDCOpen    chan struct{} //To signal when the reliable channel is open
	dcOpenWg          sync.WaitGroup    //To wait for all data channels
}

func NewSimpleWebRTCPeer(onMessage func(msg webrtc.DataChannelMessage, peer *SimpleWebRTCPeer), onClose func(peerID peer.ID)) (*SimpleWebRTCPeer, error) {
	pc, err := webrtc.NewPeerConnection(webrtcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	peer := &SimpleWebRTCPeer{
		pc:               pc,
		onMessage:        onMessage,
		onCloseCallback:  onClose,
		closeCh:          make(chan struct{}),
		state:            ConnectionStateNew,
		reliableDCOpen:   make(chan struct{}), //Initialize the new channel
	}

	//two data channels: "data" and "reliable"
	peer.dcOpenWg.Add(2)

	peer.setupConnectionHandlers()
	return peer, nil
}

func (p *SimpleWebRTCPeer) setupConnectionHandlers() {
	p.pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("ICE Connection State changed: %s", state.String())
		switch state {
		case webrtc.ICEConnectionStateConnected:
			p.setConnectionState(ConnectionStateConnected)
		case webrtc.ICEConnectionStateDisconnected:
			p.setConnectionState(ConnectionStateDisconnected)
		case webrtc.ICEConnectionStateFailed:
			p.setConnectionState(ConnectionStateFailed)
			p.Close() // Close the connection on failure
		case webrtc.ICEConnectionStateClosed:
			p.setConnectionState(ConnectionStateClosed)
			p.Close()
		}
	})

	p.pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Connection State changed: %s", state.String())
		if state == webrtc.PeerConnectionStateFailed {
			p.Close()
		}
	})

	p.pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("New DataChannel %s", dc.Label())
		if dc.Label() == "reliable" {
			p.reliableDC = dc
		} else {
			p.dc = dc
		}
		p.setupDataChannel(dc)
	})
}

func (p *SimpleWebRTCPeer) setupDataChannel(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		log.Printf("Data channel '%s' opened", dc.Label())
		p.setConnectionState(ConnectionStateConnected)

		//Signal that this data channel is open
		p.dcOpenWg.Done()

		if dc.Label() == "reliable" {
			//Specifically signal that the reliable channel is open
			close(p.reliableDCOpen)
		}
	})

	dc.OnClose(func() {
		log.Printf("Data channel '%s' closed", dc.Label())
		p.setConnectionState(ConnectionStateClosed)
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		p.onMessage(msg, p)
	})
}

func (p *SimpleWebRTCPeer) CreateOffer() (string, error) {
	// Create the unreliable data channel for file chunks
	dc, err := p.pc.CreateDataChannel("data", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create data channel: %w", err)
	}
	p.dc = dc
	p.setupDataChannel(p.dc)

	// Create the reliable data channel for control messages
	ordered := true
	maxRetransmits := uint16(5)
	reliableDC, err := p.pc.CreateDataChannel("reliable", &webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create reliable data channel: %w", err)
	}
	p.reliableDC = reliableDC
	p.setupDataChannel(p.reliableDC)

	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		return "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(p.pc)

	if err := p.pc.SetLocalDescription(offer); err != nil {
		return "", err
	}

	select {
	case <-gatherComplete:
	case <-time.After(maxICEGatheringTimeout):
		log.Println("ICE gathering timed out")
	}

	offerJSON, err := json.Marshal(p.pc.LocalDescription())
	if err != nil {
		return "", err
	}
	return string(offerJSON), nil
}

func (p *SimpleWebRTCPeer) HandleOffer(offerStr string) (string, error) {
	var offer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(offerStr), &offer); err != nil {
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

	if err := p.pc.SetLocalDescription(answer); err != nil {
		return "", err
	}

	select {
	case <-gatherComplete:
	case <-time.After(maxICEGatheringTimeout):
		log.Println("ICE gathering timed out")
	}

	answerJSON, err := json.Marshal(p.pc.LocalDescription())
	if err != nil {
		return "", err
	}
	return string(answerJSON), nil
}

func (p *SimpleWebRTCPeer) HandleAnswer(answerStr string) error {
	var answer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(answerStr), &answer); err != nil {
		return err
	}

	return p.pc.SetRemoteDescription(answer)
}

func (p *SimpleWebRTCPeer) SendJSON(v interface{}) error {
	if p.dc == nil || p.dc.ReadyState() != webrtc.DataChannelStateOpen {
		return fmt.Errorf("data channel is not open")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.dc.SendText(string(data))
}

//SendJSONReliable waits for the channel to be ready
func (p *SimpleWebRTCPeer) SendJSONReliable(v interface{}) error {
	select {
	case <-p.reliableDCOpen:
		// Channel is open, proceed
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timed out waiting for reliable data channel to open")
	case <-p.closeCh:
		return fmt.Errorf("connection closed while waiting for data channel")
	}

	if p.reliableDC == nil || p.reliableDC.ReadyState() != webrtc.DataChannelStateOpen {
		return fmt.Errorf("reliable data channel is not open")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.reliableDC.SendText(string(data))
}

func (p *SimpleWebRTCPeer) SendRaw(data []byte) error {
	if p.dc == nil || p.dc.ReadyState() != webrtc.DataChannelStateOpen {
		return fmt.Errorf("data channel is not open")
	}
	return p.dc.Send(data)
}

func (p *SimpleWebRTCPeer) Close() {
	p.closeOnce.Do(func() {
		if p.onCloseCallback != nil && p.signalingStream != nil {
			p.onCloseCallback(p.signalingStream.Conn().RemotePeer())
		}
		p.stopKeepAlive()
		if p.pc != nil {
			p.pc.Close()
		}
		if s := p.GetSignalingStream(); s != nil {
			s.Close()
		}
		close(p.closeCh)
	})
}

func (p *SimpleWebRTCPeer) SetFileWriter(w io.WriteCloser) {
	p.writerMutex.Lock()
	defer p.writerMutex.Unlock()
	p.fileWriter = w
}

func (p *SimpleWebRTCPeer) GetFileWriter() io.WriteCloser {
	p.writerMutex.RLock()
	defer p.writerMutex.RUnlock()
	return p.fileWriter
}

func (p *SimpleWebRTCPeer) SetSignalingStream(s network.Stream) {
	p.streamMux.Lock()
	defer p.streamMux.Unlock()
	p.signalingStream = s
}

func (p *SimpleWebRTCPeer) GetSignalingStream() network.Stream {
	p.streamMux.RLock()
	defer p.streamMux.RUnlock()
	return p.signalingStream
}

func (p *SimpleWebRTCPeer) WaitForConnection(timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			return fmt.Errorf("connection timeout")
		case <-ticker.C:
			if p.GetConnectionState() == ConnectionStateConnected {
				return nil
			}
		case <-p.closeCh:
			return fmt.Errorf("connection closed")
		}
	}
}

//function to wait for all data channels
func (p *SimpleWebRTCPeer) WaitForDataChannels(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		p.dcOpenWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil // All channels opened
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for data channels to open")
	case <-p.closeCh:
		return fmt.Errorf("connection closed while waiting for data channels")
	}
}

func (p *SimpleWebRTCPeer) setConnectionState(state ConnectionState) {
	p.stateMux.Lock()
	defer p.stateMux.Unlock()

	if p.state == state {
		return
	}

	p.state = state
	log.Printf("Connection state changed to: %d", state)

	if state == ConnectionStateConnected {
		p.startKeepAlive()
	} else {
		p.stopKeepAlive()
	}
}

func (p *SimpleWebRTCPeer) GetConnectionState() ConnectionState {
	p.stateMux.RLock()
	defer p.stateMux.RUnlock()
	return p.state
}

func (p *SimpleWebRTCPeer) startKeepAlive() {
	if p.keepAliveTick != nil {
		return
	}
	p.keepAliveTick = time.NewTicker(keepAliveInterval)
	go func() {
		for {
			select {
			case <-p.keepAliveTick.C:
				if err := p.SendJSON(map[string]string{"type": "ping"}); err != nil {
					log.Printf("Failed to send keepalive: %v", err)
				}
			case <-p.closeCh:
				return
			}
		}
	}()
}

func (p *SimpleWebRTCPeer) stopKeepAlive() {
	if p.keepAliveTick != nil {
		p.keepAliveTick.Stop()
		p.keepAliveTick = nil
	}
}

func (p *SimpleWebRTCPeer) WaitForCloseChannel() <-chan struct{} {
	return p.closeCh
}

func (p *SimpleWebRTCPeer) SignalDownloadComplete() {
	log.Printf("Download completed, closing connection")
	p.Close()
}

func TestICEConnectivity() error {
	pc, err := webrtc.NewPeerConnection(webrtcConfig)
	if err != nil {
		return fmt.Errorf("failed to create test connection: %w", err)
	}
	defer pc.Close()

	candidateCount := 0
	gatherComplete := make(chan struct{})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			candidateCount++
			log.Printf("Test ICE candidate: %s", candidate.String())
		}
	})

	pc.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		if state == webrtc.ICEGathererStateComplete {
			close(gatherComplete)
		}
	})

	if _, err = pc.CreateDataChannel("test", nil); err != nil {
		return fmt.Errorf("failed to create test channel: %w", err)
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("failed to create test offer: %w", err)
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("failed to set local description: %w", err)
	}

	select {
	case <-gatherComplete:
		if candidateCount == 0 {
			return fmt.Errorf("no ICE candidates generated - network connectivity issues")
		}
		return nil
	case <-time.After(maxICEGatheringTimeout):
		if candidateCount == 0 {
			return fmt.Errorf("ICE gathering timeout - no candidates generated")
		}
		log.Printf("ICE gathering timed out with %d candidates", candidateCount)
		return nil
	}
}