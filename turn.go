package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

type CloudflareIceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
}

type CloudflareTurnResponse struct {
	IceServers []CloudflareIceServer `json:"iceServers"`
}

func generateTurnToken(id string, token string) []CloudflareIceServer {
	jsonBody := []byte(`{"ttl": 86400}`)
	req, err := http.NewRequest("POST", fmt.Sprintf("https://rtc.live.cloudflare.com/v1/turn/keys/%s/credentials/generate-ice-servers", id), bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Println(err)
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(body))

	var iceServers CloudflareTurnResponse

	err = json.Unmarshal(body, &iceServers)
	if err != nil {
		panic(err)
	}

	return iceServers.IceServers
}

func turnId() string {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file")
	}
	return os.Getenv("TURN_TOKEN_ID")
}

func turnToken() string {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file")
	}
	return os.Getenv("TURN_API_TOKEN")
}
