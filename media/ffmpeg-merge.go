package media

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sunrise2575/AutoAVS/filesys"
	"github.com/tidwall/gjson"
)

func runMerge(arg *commonArgType, configJSON gjson.Result) error {
	originFolder, originName, _ := filesys.PathSplit(arg.inPath)

	mergeExt := "." + configJSON.Get("output.extension").String()

	arg.mergedPath = filepath.Join(originFolder, originName) +
		"__merged" + timestamp() + mergeExt

	// if the program has transcoded single stream and its temp file extension is same as the merge extension
	if len(arg.info) == 1 {
		for _, each := range arg.info {
			if each.outExtension == mergeExt {
				if e := os.Rename(each.outPath(), arg.mergedPath); e != nil {
					return e
				}
				return nil
			}
		}
	}

	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-y"}

	for _, eachJSON := range configJSON.Get("output.stream").Array() {
		codecType := eachJSON.Get("codec_type").String()
		if value, ok := arg.info[codecType]; ok {
			args = append(args, "-i", value.outPath())
		}
	}

	// without this line, ffmpeg transcodes again by its default settings
	args = append(args, "-c:v", "copy")
	args = append(args, "-c:a", "copy")
	args = append(args, "-c:s", "copy")
	args = append(args, arg.mergedPath)

	// merge
	cmd := exec.Command("ffmpeg", args...)
	log.Printf(`[info] Merging "%v" with arguments: %v`, arg.inPath, cmd.Args)

	stdoutStderr, e := cmd.CombinedOutput()
	if e != nil {
		return fmt.Errorf("error: %v, ffmpeg output: %v", e.Error(), string(stdoutStderr))
	}
	if len(string(stdoutStderr)) > 0 {
		log.Printf(`[info] after merging "%v", ffmpeg output: %v`, arg.inPath, string(stdoutStderr))
	}

	return nil
}

func flushFiles(arg commonArgType, configJSON gjson.Result) error {

	originFolder, originName, originExt := filesys.PathSplit(arg.inPath)
	for _, each := range arg.info {
		if _, e0 := os.Stat(each.outPath()); e0 == nil {
			if e1 := os.Remove(each.outPath()); e1 != nil {
				return e1
			}
		}
	}

	if e1 := os.Rename(arg.inPath, filepath.Join(originFolder, "."+originName)+originExt); e1 != nil {
		return e1
	}

	outPath := filepath.Join(originFolder, originName) + "." + configJSON.Get("output.extension").String()
	if e := os.Rename(arg.mergedPath, outPath); e != nil {
		return e
	}

	return nil
}
