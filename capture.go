package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/h264reader"
)

const (
	h264FrameDuration = time.Millisecond * 33
)

func CaptureScreenToTrack(ctx context.Context, track *webrtc.TrackLocalStaticSample, pc *webrtc.PeerConnection, fps int) error {

	go func() {

		dataPipe, err := RunCommand("ffmpeg",
			"-f", "gdigrab",
			"-framerate", "30",
			"-video_size", "1920x1080",
			"-i", "desktop",
			"-c:v", "h264_nvenc",
			"-preset", "12",
			//"-tune", "3", // breaks the stream
			"-rgb_mode", "yuv420",
			"-zerolatency", "1",
			"-delay", "0",
			"-qp", "50",
			"-bsf:v", "h264_mp4toannexb",
			"-b:v", "900k",
			"-bf", "0",
			"-f", "h264",
			"-b", "900k",
			"-", // important!
		)

		if err != nil {
			panic(err)
		}

		h264, h264Err := h264reader.NewReader(dataPipe)
		if h264Err != nil {
			panic(h264Err)
		}

		spsAndPpsCache := []byte{}
		ticker := time.NewTicker(h264FrameDuration)
		for ; true; <-ticker.C {
			nal, h264Err := h264.NextNAL()
			if h264Err == io.EOF {
				fmt.Printf("All video frames parsed and sent")
			} else if h264Err != nil {
				panic(h264Err)
			}

			nal.Data = append([]byte{0x00, 0x00, 0x00, 0x01}, nal.Data...)

			if nal.UnitType == h264reader.NalUnitTypeSPS || nal.UnitType == h264reader.NalUnitTypePPS {
				spsAndPpsCache = append(spsAndPpsCache, nal.Data...)
				continue
			} else if nal.UnitType == h264reader.NalUnitTypeCodedSliceIdr {
				nal.Data = append(spsAndPpsCache, nal.Data...)
				spsAndPpsCache = []byte{}
			}

			if h264Err = track.WriteSample(media.Sample{Data: nal.Data, Duration: h264FrameDuration}); h264Err != nil {
				panic(h264Err)
			}
		}
	}()

	return nil
}
