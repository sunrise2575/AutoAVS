package media

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
)

func runFFMpegTranscode(arg commonArgType, codecType string, gpuID int) error {

	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-y"}

	if codecType == "video" || codecType == "audio" {
		args = append(args,
			"-threads", "0",
			"-thread_type", "frame",
			"-analyzeduration", "2147483647",
			"-probesize", "2147483647",
		)
	}
	args = append(args, "-i", arg.inPath)

	if codecType == "video" || codecType == "audio" {
		args = append(args,
			"-threads", "0",
			"-max_muxing_queue_size", "4096",
		)
	}

	// select stream
	arg.mutex.Lock()
	args = append(args, "-map", "0:"+strconv.Itoa(arg.info[codecType].streamIndex))

	if arg.info[codecType].isTranscodeRequired {
		// transcode stream
		if codecType == "video" && gpuID >= 0 {
			args = append(args, []string{"-gpu", strconv.Itoa(gpuID)}...)
		}
		for k, v := range arg.queryJSON[codecType].Get("ffmpeg_parameter").Map() {
			switch codecType {
			case "video":
				args = append(args, []string{"-" + k + ":v", v.String()}...)
			case "audio":
				args = append(args, []string{"-" + k + ":a", v.String()}...)
			case "subtitle":
				args = append(args, []string{"-" + k + ":s", v.String()}...)
			}
		}
	} else {
		switch codecType {
		case "video":
			args = append(args, "-c:v", "copy")
		case "audio":
			args = append(args, "-c:a", "copy")
		}
	}

	arg.info[codecType] =
		transcodingInfoType{
			streamIndex:         arg.info[codecType].streamIndex,
			isTranscodeRequired: arg.info[codecType].isTranscodeRequired,
			outFolder:           arg.info[codecType].outFolder,
			outName:             arg.info[codecType].outName + timestamp() + "__" + codecType,
			outExtension:        arg.info[codecType].outExtension,
		}

	outPath := arg.info[codecType].outPath()
	arg.mutex.Unlock()

	args = append(args, outPath)

	cmd := exec.Command("ffmpeg", args...)
	log.Printf(`[info] transcoding "%v" with arguments: %v`, arg.inPath, cmd.Args)
	stdoutStderr, e := cmd.CombinedOutput()
	if e != nil {
		return fmt.Errorf("error: %v, ffmpeg output: %v", e.Error(), string(stdoutStderr))
	}
	if len(string(stdoutStderr)) > 0 {
		log.Printf(`[info] after transcoding "%v", ffmpeg output: %v`, arg.inPath, string(stdoutStderr))
	}
	return nil
}
