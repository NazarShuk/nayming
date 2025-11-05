package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return r.Header.Get("Origin") == "https://nayming.vercel.app"
	},
}

type IceServer struct {
	URLs       string `json:"urls"`
	Username   string `json:"username"`
	Credential string `json:"credential"`
}
type NaymingConfig struct {
	MouseEnabled    bool
	KeyboardEnabled bool
	StreamSettings  NaymingStreamSettings
}
type NaymingStreamSettings struct {
	FPS        int
	bitrate    int
	maxBitrate int
	minBitrate int
	qmin       int
	qmax       int
	speed      int
}

var appConfig NaymingConfig = NaymingConfig{
	MouseEnabled:    true,
	KeyboardEnabled: true,
	StreamSettings: NaymingStreamSettings{
		FPS:        60,
		bitrate:    1000,
		maxBitrate: 2000,
		minBitrate: 500,
		qmin:       10,
		qmax:       63,
		speed:      16,
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

	stop := make(chan struct{})
	defer close(stop)
	conn.SetCloseHandler(func(code int, text string) error {
		log.Println("Websocket connection was closed.")
		log.Println("Closing stop channel")
		close(stop)

		return nil
	})

	// websocket keep alive
	go func() {
		for {
			select {
			case <-stop:
				log.Println("ws ping stopped by stop channel")
				return
			default:
				// its okay
			}
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("Ping error:", err)
			}
			time.Sleep(10 * time.Second)
			log.Println("ws ping")
		}
	}()

	iceServers := generateTurnToken(turnId(), turnToken())
	conn.WriteJSON(map[string]interface{}{
		"type":       "iceServers",
		"iceServers": iceServers,
	})

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}
	for _, iceServer := range iceServers {
		config.ICEServers = append(config.ICEServers, webrtc.ICEServer{
			URLs:       iceServer.URLs,
			Username:   iceServer.Username,
			Credential: iceServer.Credential,
		})
	}

	createPeer(conn, config, &stop)

}

func createPeer(conn *websocket.Conn, config webrtc.Configuration, stop *chan struct{}) {

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Println(err)
		return
	}
	defer pc.Close()
	defer robotgo.KeyToggle("", "up")
	defer robotgo.MouseUp(robotgo.Key0)
	defer robotgo.MouseUp(robotgo.Key1)
	defer robotgo.MouseUp(robotgo.Key2)

	handlePeer(pc, stop)

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

				if !appConfig.MouseEnabled {
					return
				}
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
				case "wheel":
					robotgo.Scroll(int(mouseData["x"].(float64)), int(mouseData["y"].(float64)))
				}

			})
		}
		if dc.Label() == "keyboard" {
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				if !appConfig.KeyboardEnabled {
					return
				}
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
		select {
		case <-*stop:
			log.Println("signaling loop stopped by stop chan")
			return
		default:
			// its okay
		}
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

func handlePeer(pc *webrtc.PeerConnection, stop *chan struct{}) {
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
			select {
			case <-*stop:
				log.Println("rtcp loop stopped by stop chan")
				return
			default:
				// its okay
			}
			n, _, err := rtpSender.Read(rtcpBuf)
			if err != nil {
				log.Println("RTCP read error:", err)
				return
			}

			// Log RTCP packets for debugging
			if n > 0 {
				log.Printf("Received RTCP packet: %d bytes", n)
			}
		}
	}()

	captureErr := CaptureScreenToTrack(videoTrack, pc, appConfig.StreamSettings.FPS, stop)
	if captureErr != nil {
		log.Println(captureErr)
		return
	}
}
