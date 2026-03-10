package main

import (
	"context"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/h264reader"
)

const (
	captureFramerate  = 60
	h264FrameDuration = time.Second / captureFramerate
)

func CaptureScreenToTrack(ctx context.Context, track *webrtc.TrackLocalStaticSample, pc *webrtc.PeerConnection) error {

	go func() {

		dataPipe, err := RunCommand(ctx, "ffmpeg",
			"-init_hw_device", "d3d11va",
			"-filter_complex", "ddagrab=framerate=60:draw_mouse=0",
			"-c:v", "h264_nvenc",
			"-preset", "p1",
			//"-tune", "ull",
			"-rc", "vbr",
			"-cq", "26",
			"-maxrate", "8M",
			"-rgb_mode", "yuv420",
			"-zerolatency", "1",
			"-delay", "0",
			"-bf", "0",
			"-rc-lookahead", "0",
			"-forced-idr", "1",
			"-g", "60", // 1 keyframe per second at 60fps
			"-fflags", "nobuffer",
			"-flags", "low_delay",
			"-bsf:v", "h264_mp4toannexb",
			"-b:v", "0",
			"-f", "h264",
			"-",
		)

		if err != nil {
			panic(err)
		}

		h264, h264Err := h264reader.NewReader(dataPipe)
		if h264Err != nil {
			panic(h264Err)
		}

		var spsAndPpsCache []byte

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			nal, err := h264.NextNAL()
			if err != nil {
				return
			}

			nal.Data = append([]byte{0x00, 0x00, 0x00, 0x01}, nal.Data...)

			if nal.UnitType == h264reader.NalUnitTypeSPS || nal.UnitType == h264reader.NalUnitTypePPS {
				spsAndPpsCache = append(spsAndPpsCache, nal.Data...)
				continue
			}

			if nal.UnitType == h264reader.NalUnitTypeCodedSliceIdr {
				nal.Data = append(spsAndPpsCache, nal.Data...)
			}

			if err = track.WriteSample(media.Sample{Data: nal.Data, Duration: h264FrameDuration}); err != nil {
				return
			}
		}
	}()

	return nil
}
