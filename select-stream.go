package main

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/tidwall/gjson"
)

func selectStream(inFilePath string) (context.Context, error) {
	ffprobeBSON, e := exec.
		Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", inFilePath).
		Output()
	if e != nil {
		return nil, e
	}

	shouldEncodeVideo, shouldEncodeAudio := false, false
	vStreamIdx, aStreamIdx := 0, 0
	vIdxs, aIdxs := []int64{}, []int64{}
	vCodecName, aCodecName := "", ""
	vPixFmt := ""

	gjson.GetBytes(ffprobeBSON, "streams").ForEach(func(k, v gjson.Result) bool {
		switch v.Get("codec_type").String() {
		case "video":
			vIdxs = append(vIdxs, v.Get("index").Int())
		case "audio":
			aIdxs = append(aIdxs, v.Get("index").Int())
		}
		return true
	})

	switch len(vIdxs) {
	case 0:
		return nil, fmt.Errorf("no video stream")
	case 1:
		object := gjson.GetBytes(ffprobeBSON, "streams").Array()[vIdxs[0]]
		vStreamIdx = int(object.Get("index").Int())
		vCodecName = object.Get("codec_name").String()
		vPixFmt = object.Get("pix_fmt").String()
	default:
		// select proper stream by language
		targetStreamIdx := -1
		for _, i := range vIdxs {
			object := gjson.GetBytes(ffprobeBSON, "streams").Array()[i]

			if object.Get("tags.language").Exists() {
				if object.Get("tags.language").String() == ctxString(ctx, "language") {
					targetStreamIdx = int(object.Get("index").Int())
					break
				}
			}
		}

		if targetStreamIdx != -1 {
			vStreamIdx = targetStreamIdx
		} else {
			vStreamIdx = int(gjson.GetBytes(ffprobeBSON, "streams").Array()[vIdxs[0]].Get("index").Int())
		}

		vCodecName = gjson.GetBytes(ffprobeBSON, "streams").Array()[vStreamIdx].Get("codec_name").String()
		vPixFmt = gjson.GetBytes(ffprobeBSON, "streams").Array()[vStreamIdx].Get("pix_fmt").String()
	}

	switch len(aIdxs) {
	case 0:
		return nil, fmt.Errorf("no video stream")
	case 1:
		object := gjson.GetBytes(ffprobeBSON, "streams").Array()[aIdxs[0]]
		aStreamIdx = int(object.Get("index").Int())
		aCodecName = object.Get("codec_name").String()
	default:
		// select proper stream by language
		targetStreamIdx := -1
		for _, i := range aIdxs {
			object := gjson.GetBytes(ffprobeBSON, "streams").Array()[i]

			if object.Get("tags.language").Exists() {
				if object.Get("tags.language").String() == ctxString(ctx, "language") {
					targetStreamIdx = int(object.Get("index").Int())
					break
				}
			}
		}

		if targetStreamIdx != -1 {
			aStreamIdx = targetStreamIdx
		} else {
			aStreamIdx = int(gjson.GetBytes(ffprobeBSON, "streams").Array()[aIdxs[0]].Get("index").Int())
		}

		aCodecName = gjson.GetBytes(ffprobeBSON, "streams").Array()[aStreamIdx].Get("codec_name").String()
	}

	if !(vCodecName == "hevc" && vPixFmt == "yuv420p") {
		shouldEncodeVideo = true
	}

	if aCodecName != "aac" {
		shouldEncodeAudio = true
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, "should_encode_video", shouldEncodeVideo)
	ctx = context.WithValue(ctx, "should_encode_audio", shouldEncodeAudio)
	ctx = context.WithValue(ctx, "video_stream_index", vStreamIdx)
	ctx = context.WithValue(ctx, "audio_stream_index", aStreamIdx)

	return ctx, nil
}
