package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/sunrise2575/AutoAVS/filesys"
	"github.com/sunrise2575/AutoAVS/media"
	"github.com/tidwall/gjson"
)

// CheckGPU ...
func checkNvidiaGPU() (int, error) {
	pci, e := ghw.PCI()
	if e != nil {
		return -1, e
	}

	devices := pci.ListDevices()
	if len(devices) == 0 {
		return -1, fmt.Errorf("no PCI devices in your computer")
	}

	nvidiaGPUs := 0
	for _, device := range devices {
		if device.Vendor.Name == "NVIDIA Corporation" && !strings.Contains(device.Product.Name, "Audio") {
			fmt.Println(device)
			nvidiaGPUs++
		}
	}

	return nvidiaGPUs, nil
}

func readJSONFile(filePath string) (gjson.Result, error) {
	b, e := ioutil.ReadFile(filePath)
	if e != nil {
		return gjson.Result{}, e
	}
	return gjson.ParseBytes(b), nil
}

func main() {
	inFolderPath, configPath := "", ""
	workersPerGPU := 0

	flag.StringVar(&inFolderPath, "root", "", "Root path for input files")
	flag.StringVar(&configPath, "config", "", "Config file path.")
	flag.IntVar(&workersPerGPU, "worker", 1, "Workers Per GPU")
	flag.Parse()

	if len(inFolderPath) == 0 || len(configPath) == 0 {
		flag.Usage()
		log.Fatalf("[fatal] You should provide a root path and config path of videos.")
	}

	// Get input parameters from program flags
	inFolderPath = filesys.PathBeautify(inFolderPath)
	configPath = filesys.PathBeautify(configPath)

	// Read config file
	config, e := readJSONFile(configPath)
	if e != nil {
		log.Fatalf("[fatal] %v\n", e.Error())
	}

	if e := media.CheckConfigSanity(config); e != nil {
		log.Fatalf("[fatal] %v\n", e.Error())
	}

	// Count NVIDIA devices (but doesn't check NVENC capability)
	nvidiaGPUCount, e := checkNvidiaGPU()
	if e != nil {
		log.Fatalf("[fatal] %v\n", e.Error())
	}
	if nvidiaGPUCount <= 0 {
		log.Println("[info] Your system does not have NVIDIA NVENC capable GPUs. Proceed to use only CPU")
	}

	// Convert array to map for extension lookup
	inputExtension := make(map[string]struct{})
	for _, v := range config.Get("input.extension").Array() {
		inputExtension[v.String()] = struct{}{}
	}

	log.Printf(`[info] start transcoding process for folder "%v" using query "%v"`, inFolderPath, configPath)

	elapsedStart := time.Now()
	erroredFiles, completeFiles := int64(0), int64(0)

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
		if nvidiaGPUCount > 0 {
			for workerID := 0; workerID < nvidiaGPUCount*workersPerGPU; workerID++ {
				wg.Add(1)
				go func(workerID int, inChan <-chan string) {
					defer wg.Done()

					gpuID := workerID / workersPerGPU

					for inFilePath := range inChan {
						log.Printf(`[info][worker %v @ GPU %v] "%v" start`, workerID, gpuID, inFilePath)
						if e := media.Transcode(inFilePath, config, gpuID); e != nil {
							log.Printf(`[error][worker %v @ GPU %v] "%v" error, %v`, workerID, gpuID, inFilePath, e.Error())
							atomic.AddInt64(&erroredFiles, 1)
						} else {
							log.Printf(`[info][worker %v @ GPU %v] "%v" finished`, workerID, gpuID, inFilePath)
							atomic.AddInt64(&completeFiles, 1)
						}
					}
				}(workerID, jobChan)
			}
		} else {
			for workerID := 0; workerID < workersPerGPU; workerID++ {
				wg.Add(1)
				go func(workerID int, inChan <-chan string) {
					defer wg.Done()

					for inFilePath := range inChan {
						log.Printf(`[info][worker %v @ CPU] "%v" start`, workerID, inFilePath)
						if e := media.Transcode(inFilePath, config, -1); e != nil {
							log.Printf(`[error][worker %v @ CPU] "%v" error, %v`, workerID, inFilePath, e.Error())
							atomic.AddInt64(&erroredFiles, 1)
						} else {
							log.Printf(`[info][worker %v @ CPU] "%v" finished`, workerID, inFilePath)
							atomic.AddInt64(&completeFiles, 1)
						}
					}
				}(workerID, jobChan)
			}
		}

		wg.Wait()
	}

	elapsedTime := time.Since(elapsedStart)
	totalFiles := erroredFiles + completeFiles
	log.Printf(`[info] complete transcoding process for folder "%v" using query "%v"`, inFolderPath, configPath)
	log.Printf(`[info] total elapsed time: %v, complete file: %v (%.3f%%), errored file: %v (%.3f%%), total file: %v`,
		elapsedTime,
		completeFiles, 100*float32(completeFiles)/float32(totalFiles),
		erroredFiles, 100*float32(erroredFiles)/float32(totalFiles),
		totalFiles)
}
