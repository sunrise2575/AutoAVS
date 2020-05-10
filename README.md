# AutoTranscoder
Transcoding Automation Project

This project is experimental now. Please be generous for little failure.

(but I think this program is quite reliable.)

## Requirements
### Related to Language 
- go

### Related to GPU
- NVIDIA graphics driver (you should see the result of `nvidia-smi` command)
- NVIDIA graphics card which can encode HEVC (if you want to encode H264, you need to modify source little bit). You can see the list of HEVC-available devices here: https://developer.nvidia.com/video-encode-decode-gpu-support-matrix
(Note: the "Max # of concurrent sessions" is fake. GPU can have multiple encoding/decoding session over this limit. This is solved by the following patch)
- NVIDIA NVENC limitation unlock patch https://github.com/keylase/nvidia-patch (which unlocks NVENC's software limitation of concurrent encoding session. If you don't patch, you can't run multiple session even you have multiple GPUs!)

### Related to Transcoding
- ffmpeg

## How to use
1. Video files in the directory will be encoded by ffmpeg parameter loaded from `config.json` file. The default `config.json` file is included in this package.
2. This program recursively found video files under the root folder designated by `-in` flag.
3. Build.
```bash
go build .
```
4. Show help.
```bash
./AutoTranscoder
```
5. Run the program, sit back and watch. For example:
```bash
./AutoTranscoder -config ./config.json -in ../myvideo/ -lang jpn -workers 3
```
