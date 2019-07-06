package main

import (
	"fmt";
	"flag";
	"os";
	"os/exec"
	"strconv"
	"strings"
	"path/filepath";
	"sync";
	"time";
	"runtime";
	"io/ioutil";
	"github.com/buger/jsonparser";
	"github.com/jaypipes/ghw";
	"github.com/fatih/color";
)

func timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05.000");
}

type job_t struct {
	file	string;
	config	string;
}
func ffmpeg_split(my_job job_t, gpu_num int) bool {
	var out_path_v, out_path_a, out_path_m, out_path string;

	in_dir, in_file := filepath.Split(my_job.file);
	in_ext := filepath.Ext(my_job.file);
	in_file = in_file[0:len(in_file)-len(in_ext)];

	out_path_a = filepath.Join(in_dir, in_file+"aac");

	out_path_v = filepath.Join(in_dir, in_file+"_intermediate.mp4");
	out_path_a = filepath.Join(in_dir, in_file+"_intermediate.aac");
	out_path_m = filepath.Join(in_dir, in_file+"_merged.mp4");

	if in_ext == ".mp4" {
		out_path = filepath.Join(in_dir, in_file+"_converted.mp4");
	} else {
		out_path = filepath.Join(in_dir, in_file+".mp4");
	}

	json_file, err := os.Open(my_job.config);
	defer json_file.Close();
	if err != nil { fmt.Println(err); return false; }

	byte_val, err := ioutil.ReadAll(json_file);
	if err != nil { fmt.Println(err); return false; }

	args_c := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-threads", "0", "-thread_type", "frame",
		"-analyzeduration", "2147483647", "-probesize", "2147483647",
		"-i", my_job.file,
		"-threads", "0", "-max_muxing_queue_size", "1024"};

	args_v := args_c
	args_a := args_c

	args_v = append(args_v, []string{"-gpu", strconv.Itoa(gpu_num)}...);

	jsonparser.ObjectEach(byte_val,
		func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			switch string(key) {
			case "video":
				args_v = append(args_v, []string{"-map", "0:v:"+string(value)}...); break;
			case "audio":
				args_a = append(args_a, []string{"-map", "0:a:"+string(value)}...); break;
			default:
				break;
			}
			return nil;
		},
		"mapping");

	jsonparser.ObjectEach(byte_val,
		func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			args_v = append(args_v, []string{"-"+string(key)+":v", string(value)}...);
			return nil;
		},
		"setting", "video");

	jsonparser.ObjectEach(byte_val,
		func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			args_a = append(args_a, []string{"-"+string(key)+":a", string(value)}...);
			return nil;
		},
		"setting", "audio");

	args_v = append(args_v, "-an");
	args_a = append(args_v, "-vn");
	args_v = append(args_v, out_path_v);
	args_a = append(args_a, out_path_a);

	chan_v := make(chan bool, 1);
	chan_a := make(chan bool, 1);

	go func(result chan bool) {
		defer close(result);
		cmd := exec.Command("ffmpeg", args_v...);
		stdoutStderr, err := cmd.CombinedOutput();
		if err != nil {
			fmt.Println(string(stdoutStderr));
			result <- false;
			return;
		}
		result <- true;
	}(chan_v);

	go func(result chan bool) {
		defer close(result);
		cmd := exec.Command("ffmpeg", args_a...);
		stdoutStderr, err := cmd.CombinedOutput();
		if err != nil {
			fmt.Println(string(stdoutStderr));
			result <- false;
			return;
		}
		result <- true;
	}(chan_a);

	result_v := <-chan_v;
	result_a := <-chan_a;

	if !(result_v && result_a) { return false; }

	args_m := []string{
		"-i", out_path_v,
		"-i", out_path_a,
		"-hide_banner", "-loglevel", "error", "-y",
		"-c:v", "copy", "-c:a", "copy", out_path_m}

	cmd := exec.Command("ffmpeg", args_m...);
	stdoutStderr, err := cmd.CombinedOutput();
	if err != nil {
		fmt.Println(string(stdoutStderr));
		return false;
	}

	os.Remove(out_path_v);
	os.Remove(out_path_a);
	os.Rename(out_path_m, out_path);
	return true;
}

func ffmpeg_normal(my_job job_t, gpu_num int) bool {
	var out_path string;

	in_dir, in_file := filepath.Split(my_job.file);
	in_ext := filepath.Ext(my_job.file);
	in_file = in_file[0:len(in_file)-len(in_ext)];

	if in_ext == ".mp4" {
		out_path = filepath.Join(in_dir, in_file+"_converted.mp4");
	} else {
		out_path = filepath.Join(in_dir, in_file+".mp4");
	}

	json_file, err := os.Open(my_job.config);
	defer json_file.Close();
	if err != nil { fmt.Println(err); return false; }

	byte_val, err := ioutil.ReadAll(json_file);
	if err != nil { fmt.Println(err); return false; }

	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-threads", "0", "-thread_type", "frame",
		"-analyzeduration", "2147483647", "-probesize", "2147483647",
		"-i", my_job.file,
		"-threads", "0", "-max_muxing_queue_size", "1024"};

	jsonparser.ObjectEach(byte_val,
		func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			switch string(key) {
			case "video":
				args = append(args, []string{"-map", "0:v:"+string(value)}...); break;
			case "audio":
				args = append(args, []string{"-map", "0:a:"+string(value)}...); break;
			default:
				break;
			}
			return nil;
		},
		"mapping");

	encode_video, _ := jsonparser.GetBoolean(byte_val, "encode", "video");

	if encode_video {
		args = append(args, []string{"-gpu", strconv.Itoa(gpu_num)}...);
		jsonparser.ObjectEach(byte_val,
			func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
				args = append(args, []string{"-"+string(key)+":v", string(value)}...);
				return nil;
			},
			"setting", "video");
	} else {
		args = append(args, []string{"-c:v", "copy"}...);
	}

	encode_audio, _ := jsonparser.GetBoolean(byte_val, "encode", "audio");

	if encode_audio {
		jsonparser.ObjectEach(byte_val,
			func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
				args = append(args, []string{"-"+string(key)+":a", string(value)}...);
				return nil;
			},
			"setting", "audio");
	} else {
		args = append(args, []string{"-c:a", "copy"}...);
	}

	args = append(args, out_path);

	cmd := exec.Command("ffmpeg", args...);
	stdoutStderr, err := cmd.CombinedOutput();
	if err != nil {
		fmt.Println(string(stdoutStderr));
		return false;
	}

	return true;
}

func worker(wg *sync.WaitGroup, my_id int, my_gpu int, in chan job_t) {
	defer wg.Done();
	for j := range in {

		fmt.Printf("[%s][%02d][START   ] %s, \tCONFIG: %s\n", timestamp(), my_id, j.file, j.config);
		if ffmpeg_normal(j, my_gpu) {
			color.HiGreen("[%s][%02d][SUCCESS ] %s, \tCONFIG: %s\n", timestamp(), my_id, j.file, j.config);
		} else {
			color.HiYellow("[%s][%02d][RETRY   ] %s, \tCONFIG: %s\n", timestamp(), my_id, j.file, j.config);
			if ffmpeg_split(j, my_gpu) {
				color.HiGreen("[%s][%02d][SUCCESS ] %s, \tCONFIG: %s\n", timestamp(), my_id, j.file, j.config);
			} else {
				color.HiRed("[%s][%02d][FAILURE ] %s, \tCONFIG: %s\n", timestamp(), my_id, j.file, j.config);
			}
		}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU());

	// Flags
	root := flag.String("r", "", "Root path for file searching");
	version := flag.Bool("v", false, "Print program version");
	flag.Parse();

	if *version {
		fmt.Println("Auto-encoder");
		fmt.Println("Version: 1.0.0");
		fmt.Println("Copyright 2019 Heeyong Yoon.");
		fmt.Println("All Rights Reserved.");
		return;
	}

	if *root == "" { fmt.Println("You should provide a root path of videos."); return; }

	pci, err := ghw.PCI();
	if err != nil { fmt.Println(err); return; }
	devices := pci.ListDevices();
	if len(devices) == 0 { return; }

	nvidia_gpus := 0;
	fmt.Println("NVIDIA GPU LIST");
	for _, device := range devices {
		if device.Vendor.Name == "NVIDIA Corporation" &&
			!strings.Contains(device.Product.Name, "Audio") {
				fmt.Printf("%s\t%s\t%s\n", device.Address, device.Vendor.Name, device.Product.Name);
				nvidia_gpus++;
		}
	}

	*root, _ = filepath.Abs(*root);
	fmt.Println("FIND VIDEO FILES UNDER", *root);
	var file_list []string;
	filepath.Walk(*root, func(p string, f os.FileInfo, err error) error {
		if filepath.Ext(p) == ".mp4" || filepath.Ext(p) == ".mkv" || filepath.Ext(p) == ".avi" || filepath.Ext(p) == ".wmv" {
			p, _ = filepath.Abs(p);
			file_list = append(file_list, p);
		}
		return nil
	})

	fmt.Println("ENGAGING WORKERS");
	workers := 6;

	task_chan := make(chan job_t, 10);

	var wg sync.WaitGroup;
	wg.Add(workers);

	for id := 0; id < workers; id++ {
		go worker(&wg, id, id % nvidia_gpus, task_chan);
	}

	for _, p := range file_list {
		task_chan <- job_t{p, filepath.Join(filepath.Dir(p), "config.json")};
	}
	close(task_chan);

	wg.Wait();
}
