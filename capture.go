package main

import (
	"context"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/h264reader"
)

const (
	h264FrameDuration = time.Millisecond * 33
)

func CaptureScreenToTrack(ctx context.Context, track *webrtc.TrackLocalStaticSample, pc *webrtc.PeerConnection) error {

	go func() {

		dataPipe, err := RunCommand(ctx, "ffmpeg",
			"-init_hw_device", "d3d11va", // Initialize D3D11 hardware acceleration
			"-filter_complex", "ddagrab=0", // Use Desktop Duplication API
			"-c:v", "h264_nvenc",
			"-preset", "p1",
			//"-tune", "ull",
			"-rgb_mode", "yuv420",
			"-zerolatency", "1",
			"-delay", "0",
			"-bf", "0", // No B-frames
			"-rc-lookahead", "0", // Disable lookahead
			"-forced-idr", "1", // Force IDR frames
			"-fflags", "nobuffer", // Reduce FFmpeg input/output buffering
			"-flags", "low_delay", // Signal low delay to the muxer
			"-qp", "25",
			"-bsf:v", "h264_mp4toannexb",
			"-b:v", "900k",
			"-bf", "0",
			"-f", "h264",
			"-", // important!
		)

		if err != nil {
			panic(err)
		}

		h264, h264Err := h264reader.NewReader(dataPipe)
		if h264Err != nil {
			panic(h264Err)
		}

		var spsAndPpsCache []byte

		// 2. NO TICKER: Process NALs as fast as FFmpeg provides them
		for {
			select {
			case <-ctx.Done():
				return
			default:
				nal, err := h264.NextNAL()
				if err != nil {
					return
				}

				// Prepend start code for Annex-B
				nal.Data = append([]byte{0x00, 0x00, 0x00, 0x01}, nal.Data...)

				if nal.UnitType == h264reader.NalUnitTypeSPS || nal.UnitType == h264reader.NalUnitTypePPS {
					spsAndPpsCache = append(spsAndPpsCache, nal.Data...)
					continue // Cache metadata, don't write sample yet
				}

				if nal.UnitType == h264reader.NalUnitTypeCodedSliceIdr {
					// 3. Prepend cached headers to EVERY IDR frame.
					// Do NOT clear the cache so new frames are always valid
					nal.Data = append(spsAndPpsCache, nal.Data...)
				}

				// 4. Write the sample. duration: 33ms for 30fps
				if err = track.WriteSample(media.Sample{Data: nal.Data, Duration: h264FrameDuration}); err != nil {
					return
				}
			}
		}
	}()

	return nil
}
