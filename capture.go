package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os/exec"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

func CaptureScreenToTrack(track *webrtc.TrackLocalStaticSample, pc *webrtc.PeerConnection, fps int, stop *chan struct{}) error {
	cmd := exec.Command("ffmpeg",
		"-f", "gdigrab",
		"-framerate", fmt.Sprintf("%d", fps),
		"-video_size", "1920x1080", // Resolution of that screen
		"-i", "desktop", // Capture desktop
		"-c:v", "libvpx",
		"-deadline", "realtime",
		"-cpu-used", "16", // Max speed (0-16 for VP9)
		"-threads", "8",
		"-error-resilient", "1",
		"-auto-alt-ref", "0",
		"-lag-in-frames", "0",
		"-b:v", "5000k", // Lower bitrate for less lag
		"-minrate", "1000k",
		"-maxrate", "5000k",
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
		"-loglevel", "error", // hide a bunch of stuff it spits out
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
		select {
		case <-*stop:
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
			cmd.Wait()
			log.Println("ffmpeg stopped by stop channel")
			return
		default:
			// its okay
		}
		defer func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
			cmd.Wait()
			log.Println("FFmpeg process stopped")
		}()

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
			if frameCount%200 == 0 {
				elapsed := time.Since(startTime).Seconds()
				actualFPS := float64(frameCount) / elapsed
				log.Printf("Sent %d frames, actual FPS: %.2f", frameCount, actualFPS)
			}
		}
	}()

	return nil
}
