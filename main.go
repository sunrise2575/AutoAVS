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

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// Flags
	var rootFolder, configPath, lang string
	var printVersion bool
	var workersPerGPU int

	flag.StringVar(&rootFolder, "in", "", "Root path for input files")
	flag.StringVar(&configPath, "config", "./config.json", "Config file path.")
	flag.BoolVar(&printVersion, "v", false, "Print program version")
	flag.IntVar(&workersPerGPU, "workers", 2, "Workers per GPU. If you have good GPU, it's okay to raise this value up to 4~6. If you have normal GPU, just set this value within 2~3.")
	flag.StringVar(&lang, "lang", "jpn", "Preferred audio/video language stream.")
	flag.Parse()

	if printVersion {
		fmt.Println("AutoTranscoder")
		fmt.Println("Version: 1.2.2")
		fmt.Println("Copyright 2020 Heeyong Yoon.")
		fmt.Println("All Rights Reserved.")
		os.Exit(0)
	}

	if rootFolder == "" {
		fmt.Println("You should provide a root path of videos.")
		flag.Usage()
		os.Exit(1)
	}

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

	ctx = context.WithValue(ctx, "input_root_folder_path", rootFolder)
	ctx = context.WithValue(ctx, "config_path", configPath)
	ctx = context.WithValue(ctx, "language", lang)
	ctx = context.WithValue(ctx, "nvidia_gpu_count", nvidiaGPUs)
	ctx = context.WithValue(ctx, "workers_per_gpu", workersPerGPU)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()

	inRootFolder, e := filepath.Abs(ctxString(ctx, "input_root_folder_path"))
	if e != nil {
		panic(e)
	}
	if _, e = os.Stat(inRootFolder); os.IsNotExist(e) {
		panic(e)
	}

	configPath, e := filepath.Abs(ctxString(ctx, "config_path"))
	if e != nil {
		panic(e)
	}

	if _, e = os.Stat(inRootFolder); os.IsNotExist(e) {
		panic(e)
	}

	gpuCount := ctxInt(ctx, "nvidia_gpu_count")
	workersPerGPU := ctxInt(ctx, "workers_per_gpu")

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
					case ".asf":
						fallthrough
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
						/*
							configPath := filepath.Join(inFileDir, "config.json")
							if _, e := os.Stat(configPath); os.IsNotExist(e) {
								log.Println(inFilePath, e.Error(), "skip")
								return nil
							}

							if _, e := os.Stat(outFileDir); os.IsNotExist(e) {
								log.Println(outFileDir, e.Error(), "skip")
								return nil
							}
						*/

						result <- fParam{
							myInFilePath: inFilePath,
							myConfigPath: configPath,
							myOutFileDir: inFileDir,
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
						log.Println(myGPUID, p.myInFilePath, "retry", e.Error())
						if e := runFFMPEGsplit(p.myInFilePath, p.myConfigPath, p.myOutFileDir, myGPUID); e != nil {
							log.Println(fmt.Sprintln(myGPUID, p.myInFilePath, "fail", e.Error()))
							continue
						}
					}

					log.Println(myGPUID, p.myInFilePath, "success")
				}
			}(wID%gpuCount, fparamq)
		}
		wg.Wait()
	}()
}
