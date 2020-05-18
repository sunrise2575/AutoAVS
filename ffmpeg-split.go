package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
)

func mergeErrorChan(cs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	out := make(chan error)

	wg.Add(len(cs))
	for _, c := range cs {
		go func(c <-chan error) {
			for n := range c {
				out <- n
			}
			wg.Done()
		}(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func runFFMPEGsplit(inFilePath, configPath, outFileDir string, gpuID int) error {
	streamCtx, e := selectStream(inFilePath)
	if e != nil {
		return fmt.Errorf("\n" + inFilePath + ",\n" + e.Error())
	}

	shouldEncodeVideo, shouldEncodeAudio :=
		ctxBool(streamCtx, "should_encode_video"), ctxBool(streamCtx, "should_encode_audio")
	vStreamIdx, aStreamIdx :=
		ctxInt(streamCtx, "video_stream_index"), ctxInt(streamCtx, "audio_stream_index")

	_, inFileName := filepath.Split(inFilePath)
	inFileExt := filepath.Ext(inFilePath)
	inFileName = strings.TrimSuffix(inFileName, inFileExt)

	outFilePathVideo := filepath.Join(outFileDir, inFileName+"_temp.mp4")
	outFilePathAudio := filepath.Join(outFileDir, inFileName+"_temp.aac")
	outFilePathMerge := filepath.Join(outFileDir, inFileName+"_merging.mp4")

	var outFilePath string
	if inFileExt == ".mp4" {
		outFilePath = filepath.Join(outFileDir, inFileExt+"_new.mp4")
	} else {
		outFilePath = filepath.Join(outFileDir, inFileExt+".mp4")
	}

	bson, e := ioutil.ReadFile(configPath)
	if e != nil {
		return fmt.Errorf("\n" + inFilePath + ",\n" + e.Error())
	}

	argsCommon := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-y",
		"-threads", "0",
		"-thread_type", "frame",
		"-analyzeduration", "2147483647",
		"-probesize", "2147483647",
		"-i", inFilePath,
		"-threads", "0",
		"-max_muxing_queue_size", "4096"}

	runVideoChan := func() <-chan error {
		result := make(chan error, 1)
		go func() {
			defer close(result)

			argsVideo := []string{}
			copy(argsVideo, argsCommon)

			argsVideo = append(argsVideo, "-map", "0:"+strconv.Itoa(vStreamIdx))

			if shouldEncodeVideo {
				argsVideo = append(argsVideo, []string{"-gpu", strconv.Itoa(gpuID)}...)
				gjson.GetBytes(bson, "video").ForEach(func(k, v gjson.Result) bool {
					argsVideo = append(argsVideo, []string{"-" + k.String() + ":v", v.String()}...)
					return true
				})
			} else {
				argsVideo = append(argsVideo, []string{"-c:v", "copy"}...)
			}

			argsVideo = append(argsVideo, "-an")
			argsVideo = append(argsVideo, outFilePathVideo)

			cmd := exec.Command("ffmpeg", argsVideo...)
			stdoutStderr, e := cmd.CombinedOutput()
			if e != nil {
				result <- fmt.Errorf("\n" + outFilePathVideo + ",\n" + e.Error() + ",\n" + string(stdoutStderr))
				return
			}
			result <- nil
		}()

		return result
	}()

	runAudioChan := func() <-chan error {
		result := make(chan error, 1)
		go func() {
			defer close(result)

			argsAudio := []string{}
			copy(argsAudio, argsCommon)

			argsAudio = append(argsAudio, "-map", "0:"+strconv.Itoa(aStreamIdx))

			if shouldEncodeAudio {
				gjson.GetBytes(bson, "audio").ForEach(func(k, v gjson.Result) bool {
					argsAudio = append(argsAudio, []string{"-" + k.String() + ":a", v.String()}...)
					return true
				})
			} else {
				argsAudio = append(argsAudio, []string{"-c:a", "copy"}...)
			}

			argsAudio = append(argsAudio, "-vn")
			argsAudio = append(argsAudio, outFilePathAudio)

			cmd := exec.Command("ffmpeg", argsAudio...)
			stdoutStderr, e := cmd.CombinedOutput()

			if e != nil {
				result <- fmt.Errorf("\n" + outFilePathAudio + ",\n" + e.Error() + ",\n" + string(stdoutStderr))
				return
			}
			result <- nil
		}()
		return result
	}()

	for e := range mergeErrorChan(runVideoChan, runAudioChan) {
		if e != nil {
			return fmt.Errorf("\n" + inFilePath + ",\n" + e.Error())
		}
	}

	argsMerge := []string{
		"-i", outFilePathVideo,
		"-i", outFilePathAudio,
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-c:v", "copy",
		"-c:a", "copy",
		outFilePathMerge}

	cmd := exec.Command("ffmpeg", argsMerge...)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("\n" + outFilePathMerge + ",\n" + err.Error() + ",\n" + string(stdoutStderr))
	}

	if e := os.Remove(outFilePathVideo); e != nil {
		return fmt.Errorf("\n" + inFilePath + ",\n" + e.Error())
	}

	if e := os.Remove(outFilePathAudio); e != nil {
		return fmt.Errorf("\n" + inFilePath + ",\n" + e.Error())
	}

	if e := os.Rename(outFilePathMerge, outFilePath); e != nil {
		return fmt.Errorf("\n" + inFilePath + ",\n" + e.Error())
	}

	if inFileExt == ".mp4" {
		if e := os.Rename(inFilePath, inFilePath+".old"); e != nil {
			return fmt.Errorf("\n" + inFilePath + ",\n" + e.Error())
		}

		if e := os.Rename(outFilePath, strings.Replace(outFilePath, "_new.mp4", ".mp4", -1)); e != nil {
			return fmt.Errorf("\n" + inFilePath + ",\n" + e.Error())
		}
	}

	return nil
}
