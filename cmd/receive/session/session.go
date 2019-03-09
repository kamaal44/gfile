package session

import (
	"fmt"
	"io"

	"github.com/Antonito/gfile/pkg/utils"
	"github.com/pions/webrtc"
)

type Session struct {
	stream         io.Writer
	peerConnection *webrtc.PeerConnection

	msgChannel chan webrtc.DataChannelMessage
	done       chan struct{}
}

// NewSession returns a new Receiver Session
func NewSession(f io.Writer) *Session {
	return &Session{
		stream:     f,
		msgChannel: make(chan webrtc.DataChannelMessage, 4096),
		done:       make(chan struct{}),
	}
}

// Connect starts a connection and waits till it ends
func (s *Session) Connect() error {
	if err := s.createConnection(); err != nil {
		return err
	}
	s.createDataHandler()

	if err := s.readOffer(); err != nil {
		return err
	}
	if err := s.createAnswer(); err != nil {
		return err
	}

	// Handle data
	for {
		select {
		case <-s.done:
			return nil
		case msg := <-s.msgChannel:
			if _, err := s.stream.Write(msg.Data); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}
	}
}

func (s *Session) createConnection() error {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return err
	}
	s.peerConnection = peerConnection

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	return nil
}

func (s *Session) readOffer() error {
	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}

	for {
		encoded, err := utils.MustReadStdin()
		if err == nil {
			if err := utils.Decode(encoded, &offer); err == nil {
				break
			}
		}
		fmt.Println("Invalid SDP, try again...")
	}

	// Set the remote SessionDescription
	return s.peerConnection.SetRemoteDescription(offer)
}

func (s *Session) createAnswer() error {
	// Create an answer
	answer, err := s.peerConnection.CreateAnswer(nil)
	if err != nil {
		return err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err = s.peerConnection.SetLocalDescription(answer); err != nil {
		return err
	}

	// Output the answer in base64 so we can paste it in browser
	resp, err := utils.Encode(answer)
	if err != nil {
		return err
	}
	fmt.Println(resp)
	return nil
}

func (s *Session) createDataHandler() {
	s.peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label, d.ID)
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			s.msgChannel <- msg
		})

		d.OnClose(func() {
			fmt.Println("Done !")
			close(s.done)
		})
	})
}