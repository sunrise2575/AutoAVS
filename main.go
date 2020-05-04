package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/jaypipes/ghw"
)

var (
	ctx = context.Background()
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Flags
	var rootFolder string
	var printVersion bool
	var workersPerGPU int
	flag.StringVar(&rootFolder, "r", "", "Root path for input files")
	flag.BoolVar(&printVersion, "v", false, "Print program version")
	flag.IntVar(&workersPerGPU, "w", 2, "Workers per GPU. If you have good GPU, it's okay to raise this value up to 4~6. If you have normal GPU, just set this value within 2~3. Default value is 2")
	flag.Parse()

	if printVersion {
		fmt.Println("Auto-encoder")
		fmt.Println("Version: 1.1.0")
		fmt.Println("Copyright 2020 Heeyong Yoon.")
		fmt.Println("All Rights Reserved.")
		os.Exit(0)
	}

	if rootFolder == "" {
		fmt.Println("You should provide a root path of videos.")
		flag.Usage()
		os.Exit(1)
	}

	ctx = context.WithValue(ctx, "input_root_folder_path", rootFolder)

	pci, e := ghw.PCI()
	if e != nil {
		panic(e)
	}

	devices := pci.ListDevices()
	if len(devices) == 0 {
		panic("no PCI devices!")
	}

	nvidiaGPUs := 0
	fmt.Println("NVIDIA GPU LIST")
	for _, device := range devices {
		if device.Vendor.Name == "NVIDIA Corporation" && !strings.Contains(device.Product.Name, "Audio") {
			fmt.Printf("%s\t%s\t%s\n", device.Address, device.Vendor.Name, device.Product.Name)
			nvidiaGPUs++
		}
	}

	ctx = context.WithValue(ctx, "nvidia_gpu_count", nvidiaGPUs)

	ctx = context.WithValue(ctx, "workers_per_gpu", workersPerGPU)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()

	// Load root folder path
	var inRootFolder string
	if v := ctx.Value("input_root_folder_path"); v != nil {
		if u, ok := v.(string); !ok {
			panic("context error")
		} else {
			var e error
			if inRootFolder, e = filepath.Abs(u); e != nil {
				panic(e)
			}
			if _, e = os.Stat(inRootFolder); os.IsNotExist(e) {
				panic(e)
			}
		}
	}

	var gpuCount int
	if v := ctx.Value("nvidia_gpu_count"); v != nil {
		if u, ok := v.(int); !ok {
			panic("context error")
		} else {
			gpuCount = u
		}
	}

	var workersPerGPU int
	if v := ctx.Value("workers_per_gpu"); v != nil {
		if u, ok := v.(int); !ok {
			panic("context error")
		} else {
			workersPerGPU = u
		}
	}

	type fParam struct {
		myInFilePath, myConfigPath, myOutFileDir string
	}

	// producer
	fparamq := func() <-chan fParam {
		result := make(chan fParam, 128)
		go func() {
			defer func() {
				close(result)
				if r := recover(); r != nil {
					log.Println(r)
				}
			}()

			if e := filepath.Walk(inRootFolder, func(inFilePath string, info os.FileInfo, err error) error {
				if !info.IsDir() {
					switch filepath.Ext(inFilePath) {
					case ".mp4":
						fallthrough
					case ".mkv":
						fallthrough
					case ".avi":
						fallthrough
					case ".wmv":
						inFilePath, e := filepath.Abs(inFilePath)
						if e != nil {
							log.Println(inFilePath, e.Error(), "skip")
							return nil
						}

						inFileDir, _ := filepath.Split(inFilePath)
						configPath := filepath.Join(inFileDir, "config.json")
						outFileDir := inFileDir

						if _, e := os.Stat(configPath); os.IsNotExist(e) {
							log.Println(inFilePath, e.Error(), "skip")
							return nil
						}

						if _, e := os.Stat(outFileDir); os.IsNotExist(e) {
							log.Println(outFileDir, e.Error(), "skip")
							return nil
						}

						result <- fParam{
							myInFilePath: inFilePath,
							myConfigPath: configPath,
							myOutFileDir: outFileDir,
						}
					}
				}
				return nil
			}); e != nil {
				panic(e)
			}
		}()
		return result
	}()

	// consumer
	func() {
		var wg sync.WaitGroup
		for wID := 0; wID < gpuCount*workersPerGPU; wID++ {
			wg.Add(1)
			go func(myGPUID int, in <-chan fParam) {
				defer func() {
					if r := recover(); r != nil {
						log.Println(r)
					}
					wg.Done()
				}()

				for p := range in {
					log.Println(myGPUID, p.myInFilePath, "start")

					if e := runFFMPEG(p.myInFilePath, p.myConfigPath, p.myOutFileDir, myGPUID); e != nil {
						log.Println(myGPUID, p.myInFilePath, "retry")
						if e := runFFMPEGsplit(p.myInFilePath, p.myConfigPath, p.myOutFileDir, myGPUID); e != nil {
							panic(fmt.Sprintln(myGPUID, p.myInFilePath, "fail"))
						}
					}

					log.Println(myGPUID, p.myInFilePath, "success")
				}
			}(wID%gpuCount, fparamq)
		}
		wg.Wait()
	}()
}
