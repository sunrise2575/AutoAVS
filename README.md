# Auto-Encoder
Encoding Automation Project (not machine learning!)

This project is experimental now. Please be generous for encoding failure.

## Requirements
- Python 3.5 (or more)
- aiofiles(https://github.com/Tinche/aiofiles)

- NVIDIA graphics driver (you should see the result of `nvidia-smi` command)
- NVIDIA graphics card which can encode HEVC (if you want to encode H264, you need to modify source little bit). You can see the list of HEVC-available devices here: https://developer.nvidia.com/video-encode-decode-gpu-support-matrix
(Note: the "Max # of concurrent sessions" is fake. GPU can have multiple encoding/decoding session over this limit. This is solved by the following patch)
- NVIDIA NVENC limitation unlock patch https://github.com/keylase/nvidia-patch (which unlocks NVENC's software limitation of concurrent encoding session. If you don't patch, you can't run multiple session even you have multiple GPUs!)

## How to use
1. You must write down and save `config.json` file on the directory. Video files in the directory will be encoded by ffmpeg parameter loaded from `config.json` file.
2. The directory can be multiple. This program recursively found `config.json` and video files.
3. Type command:
```bash
python3 auto-encoder.py (video file directories' root directory)
```
