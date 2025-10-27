package main

import "github.com/pion/webrtc/v3"

func doneChan(pc *webrtc.PeerConnection) chan struct{} {
	done := make(chan struct{})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateDisconnected ||
			state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed {
			close(done)
		}
	})
	return done
}
