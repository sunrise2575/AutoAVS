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
		return e
	}

	argsCommon := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-threads", "0",
		"-thread_type", "frame",
		"-analyzeduration", "2147483647",
		"-probesize", "2147483647",
		"-i", inFilePath,
		"-threads", "0",
		"-max_muxing_queue_size", "1024"}

	runVideoChan := func() <-chan error {
		result := make(chan error, 1)
		go func() {
			defer close(result)

			argsVideo := []string{}
			copy(argsVideo, argsCommon)

			gjson.GetBytes(bson, "mapping.video").ForEach(func(k, v gjson.Result) bool {
				argsVideo = append(argsVideo, []string{"-map", "0:v:" + v.String()}...)
				return true
			})

			argsVideo = append(argsVideo, []string{"-gpu", strconv.Itoa(gpuID)}...)

			gjson.GetBytes(bson, "setting.video").ForEach(func(k, v gjson.Result) bool {
				argsVideo = append(argsVideo, []string{"-" + k.String() + ":v", v.String()}...)
				return true
			})

			argsVideo = append(argsVideo, "-an")
			argsVideo = append(argsVideo, outFilePathVideo)

			cmd := exec.Command("ffmpeg", argsVideo...)
			stdoutStderr, e := cmd.CombinedOutput()
			if e != nil {
				result <- fmt.Errorf(e.Error() + string(stdoutStderr))
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

			gjson.GetBytes(bson, "mapping.audio").ForEach(func(k, v gjson.Result) bool {
				argsAudio = append(argsAudio, []string{"-map", "0:a:" + v.String()}...)
				return true
			})

			gjson.GetBytes(bson, "setting.audio").ForEach(func(k, v gjson.Result) bool {
				argsAudio = append(argsAudio, []string{"-" + k.String() + ":a", v.String()}...)
				return true
			})

			argsAudio = append(argsAudio, "-vn")
			argsAudio = append(argsAudio, outFilePathAudio)

			cmd := exec.Command("ffmpeg", argsAudio...)
			stdoutStderr, e := cmd.CombinedOutput()

			if e != nil {
				result <- fmt.Errorf(e.Error() + string(stdoutStderr))
				return
			}
			result <- nil
		}()
		return result
	}()

	for e := range mergeErrorChan(runVideoChan, runAudioChan) {
		if e != nil {
			return e
		}
	}

	argsMerge := []string{
		"-i", outFilePathVideo,
		"-i", outFilePathAudio,
		"-hide_banner", "-loglevel", "error", "-y",
		"-c:v", "copy", "-c:a", "copy", outFilePathMerge}

	cmd := exec.Command("ffmpeg", argsMerge...)
	stdoutStderr, e := cmd.CombinedOutput()
	if e != nil {
		return fmt.Errorf(e.Error() + string(stdoutStderr))
	}

	if e := os.Remove(outFilePathVideo); e != nil {
		return e
	}

	if e := os.Remove(outFilePathAudio); e != nil {
		return e
	}

	if e := os.Rename(outFilePathMerge, outFilePath); e != nil {
		return e
	}

	return nil
}
