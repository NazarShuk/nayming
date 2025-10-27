package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-vgo/robotgo"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
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

				switch mouseData["type"] {
				case "down":
					switch mouseData["button"] {
					case "left":
						robotgo.MouseDown(robotgo.Left)
					case "right":
						robotgo.MouseDown(robotgo.Right)
					}
				case "up":
					switch mouseData["button"] {
					case "left":
						robotgo.MouseUp(robotgo.Left)
					case "right":
						robotgo.MouseUp(robotgo.Right)
					}
				case "move":
					robotgo.Move(int(mouseData["x"].(float64)), int(mouseData["y"].(float64)))

				}

			})
		}
		if dc.Label() == "keyboard" {
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				// convert to json
				var keyboardData map[string]interface{}
				json.Unmarshal([]byte(msg.Data), &keyboardData)
				fmt.Println(keyboardData)
				switch keyboardData["type"] {
				case "down":
					robotgo.KeyDown(keyboardData["key"].(string))
				case "up":
					robotgo.KeyUp(keyboardData["key"].(string))
				}
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
