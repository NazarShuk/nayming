package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for demo
	},
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	// WebRTC configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Println(err)
		return
	}
	defer pc.Close()
	handlePeer(pc)

	// Send ICE candidates to client
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			candidateJSON, _ := json.Marshal(candidate.ToJSON())
			conn.WriteJSON(map[string]interface{}{
				"type":      "candidate",
				"candidate": string(candidateJSON),
			})
		}
	})

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == "alive" {
			fmt.Print("alive")

		}
		if dc.Label() == "mouse" {
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				// convert to json
				var mouseData map[string]interface{}
				json.Unmarshal([]byte(msg.Data), &mouseData)
				fmt.Println(mouseData)

				moveMouse(int32(mouseData["x"].(float64)), int32(mouseData["y"].(float64)))
			})
		}

	})

	// Handle signaling messages
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		switch msg["type"] {
		case "offer":
			// Set remote description
			offer := webrtc.SessionDescription{
				Type: webrtc.SDPTypeOffer,
				SDP:  msg["sdp"].(string),
			}
			if err := pc.SetRemoteDescription(offer); err != nil {
				log.Println(err)
				continue
			}

			// Create answer
			answer, err := pc.CreateAnswer(nil)
			if err != nil {
				log.Println(err)
				continue
			}

			if err := pc.SetLocalDescription(answer); err != nil {
				log.Println(err)
				continue
			}

			// Send answer back
			conn.WriteJSON(map[string]interface{}{
				"type": "answer",
				"sdp":  answer.SDP,
			})

		case "candidate":
			var candidate webrtc.ICECandidateInit
			json.Unmarshal([]byte(msg["candidate"].(string)), &candidate)
			pc.AddICECandidate(candidate)
		}
	}

}

func handlePeer(pc *webrtc.PeerConnection) {
	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		"video",
		"pion",
	)
	if err != nil {
		log.Println(err)
		return
	}
	rtpSender, videoTrackErr := pc.AddTrack(videoTrack)
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, err := rtpSender.Read(rtcpBuf); err != nil {
				return
			}
		}
	}()

	captureErr := CaptureScreenToTrack(videoTrack, 60)
	if captureErr != nil {
		log.Println(captureErr)
		return
	}
}

func CaptureScreenToTrack(track *webrtc.TrackLocalStaticSample, fps int) error {
	cmd := exec.Command("ffmpeg",
		"-f", "gdigrab",
		"-framerate", fmt.Sprintf("%d", fps),
		"-offset_x", "0", // X position of the monitor (0 for primary)
		"-offset_y", "0", // Y position of the monitor
		"-video_size", "1920x1080", // Resolution of that screen
		"-i", "desktop", // Capture desktop
		"-c:v", "libvpx",
		"-deadline", "realtime",
		"-cpu-used", "16", // Max speed (0-16 for VP9)
		"-threads", "8",
		"-error-resilient", "1",
		"-auto-alt-ref", "0",
		"-lag-in-frames", "0",
		"-b:v", "800k", // Lower bitrate for less lag
		"-minrate", "400k",
		"-maxrate", "1000k",
		"-bufsize", "400k",
		"-quality", "realtime",
		"-speed", "16", // Max speed
		"-tile-columns", "2",
		"-frame-parallel", "1",
		"-static-thresh", "0",
		"-max-intra-rate", "300",
		"-qmin", "10", // Allow more quantization (lower quality)
		"-qmax", "63", // Max compression
		"-undershoot-pct", "100",
		"-pix_fmt", "yuv420p",
		"-f", "ivf",
		"pipe:1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Println("FFmpeg:", scanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		defer cmd.Wait()

		// Read IVF header (32 bytes)
		ivfHeader := make([]byte, 32)
		if _, err := io.ReadFull(stdout, ivfHeader); err != nil {
			log.Println("Failed to read IVF header:", err)
			return
		}

		frameDuration := time.Second / time.Duration(fps)
		frameCount := 0
		startTime := time.Now()

		for {
			// Read frame header (12 bytes)
			frameHeader := make([]byte, 12)
			if _, err := io.ReadFull(stdout, frameHeader); err != nil {
				if err == io.EOF {
					log.Println("Stream ended")
					return
				}
				log.Println("Frame header error:", err)
				return
			}

			// Extract frame size (little-endian, bytes 0-3)
			frameSize := uint32(frameHeader[0]) |
				uint32(frameHeader[1])<<8 |
				uint32(frameHeader[2])<<16 |
				uint32(frameHeader[3])<<24

			// Read frame data
			frameData := make([]byte, frameSize)
			if _, err := io.ReadFull(stdout, frameData); err != nil {
				log.Println("Frame data error:", err)
				return
			}

			// Write to track immediately (no buffering)
			if err := track.WriteSample(media.Sample{
				Data:     frameData,
				Duration: frameDuration,
			}); err != nil {
				log.Println("Track write error:", err)
			}

			frameCount++
			if frameCount%30 == 0 {
				elapsed := time.Since(startTime).Seconds()
				actualFPS := float64(frameCount) / elapsed
				log.Printf("Sent %d frames, actual FPS: %.2f", frameCount, actualFPS)
			}
		}
	}()

	return nil
}
