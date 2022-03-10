package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"

	"github.com/sunrise2575/AutoAVS/filesys"
	"github.com/tidwall/gjson"
)

func decideResolution(value int) string {
	middle := func(a, b int) int {
		return (a + b) / 2
	}

	switch {
	case value <= middle(240, 360):
		return "lo_res"
	case middle(240, 360) < value && value <= middle(360, 480):
		return "360p"
	case middle(360, 480) < value && value <= middle(480, 720):
		return "480p"
	case middle(480, 720) < value && value <= middle(720, 1080):
		return "720p"
	case middle(720, 1080) < value && value <= middle(1080, 1440):
		return "1080p"
	default:
		return "hi_res"
	}
}

func moveFile(outMotherFolder string, inFilePath string, width, height int) {
	shape, res := "", ""

	switch {
	case width > height:
		shape = "horizontal"
		res = decideResolution(height)
	case width < height:
		shape = "vertical"
		res = decideResolution(width)
	default:
		shape = "square"
		res = fmt.Sprintf("%vx%v", width, height)
	}

	inFilePath = filesys.PathBeautify(inFilePath)
	_, inFileName := path.Split(inFilePath)

	newFolderPath := path.Join(outMotherFolder, shape, res+"/")
	os.MkdirAll(newFolderPath, 0755)
	newFilePath := path.Join(newFolderPath, inFileName)

	//fmt.Printf("%v -> (%v, %v) -> (%v, %v) -> %v\n", inFilePath, width, height, shape, res, newFilePath)
	fmt.Printf("%v -> %v\n", inFilePath, newFilePath)
	os.Rename(inFilePath, newFilePath)
}

func main() {
	inFolderPath := ""
	flag.StringVar(&inFolderPath, "root", "", "Root path for input files")
	flag.Parse()

	if len(inFolderPath) == 0 {
		flag.Usage()
		log.Fatalf("[fatal] You should provide a root path and config path of videos.")
	}

	inFolderPath = filesys.PathBeautify(inFolderPath)
	outFolderPath := path.Join(inFolderPath, ".sorted/")

	inputExtension := make(map[string]struct{})
	for _, v := range []string{"mkv", "mp4", "avi", "webm"} {
		inputExtension[v] = struct{}{}
	}

	// Producer
	jobChan := make(chan string, 128)

	go func() {
		defer close(jobChan)
		filepath.Walk(inFolderPath, func(inFilePath string, info os.FileInfo, err error) error {
			// exclude if selected path is not file
			if info.IsDir() {
				return nil
			}

			// exclude if the selected file has no extension
			if len(filepath.Ext(inFilePath)) < 2 {
				return nil
			}

			// exclude if the file's extension is not in the list that we want to target
			if _, ok := inputExtension[filepath.Ext(inFilePath)[1:]]; !ok {
				return nil
			}

			// split the path three way
			folder, name, _ := filesys.PathSplit(inFilePath)

			// transcoded file leaves dot-prefix named file
			// therefore, exclude if the file is a dot-prefix named file
			if len(name) > 0 && name[0] == '.' {
				return nil
			}

			// transcoded file leaves dot-prefix named file
			// therefore, exclude if the file has already left a dot-prefix named file
			dotPrefixNamedRegex := filepath.Join(folder, "."+name) + ".*"
			matches, _ := filepath.Glob(dotPrefixNamedRegex)
			if len(matches) > 0 {
				return nil
			}

			// if the file is alright, insert the file in the queue
			jobChan <- inFilePath

			// leave lambda function
			return nil
		})
	}()

	// Consumer
	{
		var wg sync.WaitGroup
		for workerID := 0; workerID < 6; workerID++ {
			wg.Add(1)
			go func(workerID int, inChan <-chan string) {
				defer wg.Done()

				for inFilePath := range inChan {
					if !filesys.IsFile(inFilePath) {
						continue
					}

					// get info
					result, e := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", inFilePath).Output()
					if e != nil {
						continue
					}

					width, height := 0, 0
					//length := 0.0

					// group-by existing stream in the media file
					for _, v := range gjson.ParseBytes(result).Get("streams").Array() {
						if v.Get("codec_type").String() == "video" {
							width = int(v.Get("coded_width").Int())
							height = int(v.Get("coded_height").Int())
							//length = v.Get("duration").Float()
							break
						}
					}

					if width > 0 && height > 0 {
						moveFile(outFolderPath, inFilePath, width, height)
					}

				}
			}(workerID, jobChan)
		}

		wg.Wait()
	}
}
