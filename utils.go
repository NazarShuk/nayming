package main

import (
	"bufio"
	"io"
	"log"
	"os/exec"

	"github.com/pion/webrtc/v3"
)

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
func RunCommand(name string, arg ...string) (io.ReadCloser, error) {
	cmd := exec.Command(name, arg...)

	dataPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Println("FFmpeg:", scanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return dataPipe, nil
}
