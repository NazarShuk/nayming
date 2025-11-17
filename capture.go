package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var (
	// Pool for IVF headers (32 bytes)
	headerPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 32)
			return &buf
		},
	}

	// Pool for frame headers (12 bytes)
	frameHeaderPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 12)
			return &buf
		},
	}

	// Pool for frame data (reusable slices)
	// We'll use a tiered approach for different sizes
	smallFramePool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 32*1024) // 32KB
			return &buf
		},
	}

	mediumFramePool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 128*1024) // 128KB
			return &buf
		},
	}

	largeFramePool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 512*1024) // 512KB
			return &buf
		},
	}
)

func getFrameBuffer(size uint32) (*[]byte, *sync.Pool) {
	var pool *sync.Pool

	switch {
	case size <= 32*1024:
		pool = &smallFramePool
	case size <= 128*1024:
		pool = &mediumFramePool
	default:
		pool = &largeFramePool
	}

	bufPtr := pool.Get().(*[]byte)
	buf := *bufPtr

	// If pooled buffer is too small, allocate a new one
	if uint32(len(buf)) < size {
		newBuf := make([]byte, size)
		return &newBuf, nil // Don't return to pool
	}

	// Slice to exact size needed
	buf = buf[:size]
	*bufPtr = buf
	return bufPtr, pool
}

func CaptureScreenToTrack(ctx context.Context, track *webrtc.TrackLocalStaticSample, pc *webrtc.PeerConnection, fps int) error {
	stream := ffmpeg.Input("desktop",
		ffmpeg.KwArgs{
			"f":          "gdigrab",
			"framerate":  fmt.Sprintf("%d", fps),
			"video_size": "1920x1080",
		}).
		Output("pipe:",
			ffmpeg.KwArgs{
				"c:v":             "libvpx",
				"deadline":        "realtime",
				"cpu-used":        "16",
				"threads":         "8",
				"error-resilient": "1",
				"auto-alt-ref":    "0",
				"lag-in-frames":   "0",
				"b:v":             fmt.Sprintf("%dk", appConfig.StreamSettings.bitrate),
				"minrate":         fmt.Sprintf("%dk", appConfig.StreamSettings.minBitrate),
				"maxrate":         fmt.Sprintf("%dk", appConfig.StreamSettings.maxBitrate),
				"bufsize":         "400k",
				"quality":         "realtime",
				"speed":           fmt.Sprintf("%d", appConfig.StreamSettings.speed),
				"tile-columns":    "2",
				"frame-parallel":  "1",
				"static-thresh":   "0",
				"max-intra-rate":  "300",
				"qmin":            fmt.Sprintf("%d", appConfig.StreamSettings.qmin),
				"qmax":            fmt.Sprintf("%d", appConfig.StreamSettings.qmax),
				"undershoot-pct":  "100",
				"pix_fmt":         "yuv420p",
				"f":               "ivf",
				"loglevel":        "error",
			})

	cmd := stream.Compile()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe error: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe error: %w", err)
	}

	// Monitor stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				log.Println("FFmpeg:", scanner.Text())
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start error: %w", err)
	}

	// Goroutine to handle FFmpeg process lifecycle
	go func() {
		defer func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
			cmd.Wait()
			log.Println("FFmpeg process stopped")
		}()

		// Read IVF header using pool
		headerBufPtr := headerPool.Get().(*[]byte)
		ivfHeader := *headerBufPtr
		defer headerPool.Put(headerBufPtr)

		if _, err := io.ReadFull(stdout, ivfHeader); err != nil {
			log.Println("Failed to read IVF header:", err)
			return
		}

		frameDuration := time.Second / time.Duration(fps)
		frameCount := 0
		startTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				log.Println("Capture stopped:", ctx.Err())
				return
			default:
			}

			// Read frame header using pool
			frameHeaderBufPtr := frameHeaderPool.Get().(*[]byte)
			frameHeader := *frameHeaderBufPtr

			if _, err := io.ReadFull(stdout, frameHeader); err != nil {
				frameHeaderPool.Put(frameHeaderBufPtr)
				if err == io.EOF {
					log.Println("Stream ended")
					return
				}
				log.Println("Frame header error:", err)
				return
			}

			// Extract frame size (little-endian)
			frameSize := uint32(frameHeader[0]) |
				uint32(frameHeader[1])<<8 |
				uint32(frameHeader[2])<<16 |
				uint32(frameHeader[3])<<24

			frameHeaderPool.Put(frameHeaderBufPtr)

			// Validate frame size
			if frameSize == 0 || frameSize > 10*1024*1024 { // Max 10MB
				log.Printf("Invalid frame size: %d bytes", frameSize)
				return
			}

			// Get appropriately sized buffer from pool
			frameBufPtr, pool := getFrameBuffer(frameSize)
			frameData := (*frameBufPtr)[:frameSize]

			if _, err := io.ReadFull(stdout, frameData); err != nil {
				if pool != nil {
					pool.Put(frameBufPtr)
				}
				log.Println("Frame data error:", err)
				return
			}

			// IMPORTANT: Make a copy for WriteSample since we're reusing the buffer
			// WebRTC might hold onto the slice asynchronously
			frameCopy := make([]byte, frameSize)
			copy(frameCopy, frameData)

			// Return buffer to pool immediately
			if pool != nil {
				pool.Put(frameBufPtr)
			}

			// Write to track
			if err := track.WriteSample(media.Sample{
				Data:     frameCopy,
				Duration: frameDuration,
			}); err != nil {
				log.Println("Track write error:", err)
				// Don't return on write errors, might be temporary
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
