# AutoAVS

The automated audio, video, subtitle tool. Currently audio and video is supported.

## When to use this program?

- Transcode hundreds of video/audio file **seamlessly** in a computer as the manner of **bulk processing**

  - Just hit Enter button, and your computer can't be idle before finishing jobs

- Select single stream from multiple streams **automatically**, transcode/copy each stream, and merge into single file

- Supports **user query** for general usage conditions (note: `query_*.json`)

  - movie, drama, anime, music, ... whatever you want to transcode

## Requirements

### Basic requirements

- golang (for program compile)

- `ffmpeg`, `ffprobe`

  - Note: I developed and tested in `ffmpeg 3.4.8-0ubuntu0.2 built with gcc 7 (Ubuntu 7.5.0-3ubuntu1~18.04)` version

- (Optional) Linux

  - I haven't tested on Windows environment, but if you replace `ghw.PCI` package to another way, you can utilize my program.

### (Optional, but recommended) requirements for using GPU

- This program can utilize `ffmpeg`'s NVENC support, so you can use your NVIDIA GPU for transcoding

  - You can see the list of NVENC devices here: https://developer.nvidia.com/video-encode-decode-gpu-support-matrix

    - You can see that older GPUs can't support much codecs.

  - This program doesn't use NVDEC for supporting various codecs without errors, i.e. decode on CPU → encode on GPU)

    - Note: NVDEC supports few codecs like H264, HEVC, VP8, VP9, AV1, ...

- NVIDIA graphics driver (you should see the result of `nvidia-smi` command)

- NVIDIA **NVENC limitation unlock** patch: https://github.com/keylase/nvidia-patch

  - That website's `Max # of concurrent sessions` is a kind of NVIDIA's fraud. It is implemented as GPU driver, so it is a software lock.

  - GPU can have capability to running multiple encoding/decoding session over this software limit. This limit can be unlocked by this patch

  - If you can't/don't patch, you can't run multiple session **even you have multiple GPUs**

## How to use

1. Write down the query in `*.json` file. Your all video/audio files under the selected directory will be processed by this query. The default `query_*.json` files are included in this package. The query is designed like:

    ```json
    {
        // "input" section
        "input": {
            // the program scans following files
            "extension": ["mp3", "mkv", "mp4", ...]
        },

        // "output" section
        "output": {
            "stream": [
              // "stream" is an array, and the order decides output media stream
              //   For example,
              //     "stream": input query = [{"codec_type":"audio", ...}, {"codec_type":"video", ...}]
              //       -> file's output stream order = ["audio", "video"]
              {
                    // "codec_type" is "video", "audio", and "subtitle"
                    "codec_type": "video",
                    // This example means that:
                    //   The 1st stream will be video

                    // You can omit "select_if" and "select_priority"
                    // Which means that you choose 0-th video stream whatever it is

                    // You can write "copy_if" here
                    "copy_if": {
                        // in this section, you can write regex JSON compare query
                        "codec_name": "^(hevc)$", 
                        "pix_fmt": "^(yuv420p)$"
                    },

                    // This stream's example section means that:
                    //   If "codec_name" is "hevc"
                    //      and "pix_fmt" is "yuv420p",
                    //      then the program just copies this stream.
                    //   Otherwise this stream should be transcoded

                    // ffmpeg transcoding parameter
                    "ffmpeg_parameter": {
                        "c": "hevc_nvenc",
                        "pix_fmt": "yuv420p",
                        "profile": "main",
                        "level": "auto",
                        "preset": "hq",
                        "qp": "25"
                        // This example section means that:
                        //   ffmpeg ... -c:v hevc_nvenc -pix_fmt:v yuv420p -profile:v main -level:v auto -preset:v hq -qp:v 25 ...
                    },


                    "temp_file_extension": "mkv" // temp file extension before merging
                },
                {
                    // "codec_type" is "video", "audio", and "subtitle"
                    "codec_type": "audio",
                    // This example means that:
                    //   The 2nd stream will be audio


                    "select_if": {
                        // in this section, you can write regex JSON compare query
                        // regex is combined golang-optional regex and perl regex

                        "channels": 2,
                        "tags": {
                            "language": "^(eng|und)$",
                            "title": "(?i)^(?!.*comment).*$"
                            // note: (?i) is golang-style regex, means insensitive
                            //   for additional options like global, multiline, ... you should use golang-style regex
                            // note: ^(?!.*comment).*$ is perl-style regex (i.e. lookahead)
                            //   for core regex, you should use perl-style regex
                        }
                    },

                    // "select_priority" is
                    "select_priority": ["tags.title", "tags.language"], // select_prioritty

                    // This stream's example section means that:
                    //   If "channels" is 2
                    //     and "language" is "eng" or "und"
                    //     and "title" is not including "comment", the program prefers to choose.
                    //   Especially the program gives most priority for "tags.title",
                    //     second priority for "tags.language"
                    //     and other JSON fields are equally same priority
                    //   Basically, choose only 1 stream for this "codec_type" from entire audio stream candidates,
                    //     therefore the highest ranked stream is selected

                    // you can omit "copy_if" section
                    // Which means that that:
                    //   All kinds of audio stream should be transcoded

                    // ffmpeg transcoding parameter
                    "ffmpeg_parameter": {
                        "c": "libopus"
                        // What this example section means is that:
                        //   ffmpeg ... -c:a libopus ...
                    },

                    "temp_file_extension": "mka" // temp file extension before merging
                }
            ],

            // final output extension
            //   If the number of requested stream in query is just one,
            //     and its "temp_file_extension" and this "extension" is same,
            //     then the merging process is skipped, because its behavior is just useless file copying.
            "extension": "mp4"
        }
    }
    ```

    Note: `"codec_type"`, `"select_if"` and `"copy_if"` compares its value to the result of following command:

    ```bash
      ffprobe -v quiet -print_format json -show_streams <filename>
    ```


2. Run the program by following command, sit back and watch

    (Note: This program recursively found video files under the root folder designated by `-root` flag.)
    
    ```bash
    go run . -config <query_JSON_file> -root <your_video_path> -worker <workers>
    ```

## Note

- This program has been developed for my personal usage. Please be generous for little failure. (But I personally think the program is quite reliable.) You can submit some push request for fixing bugs and helping me.

- Also, you can fork my repostory, but please notify me when not using this program in personal way (e.g. commercial use, integrate to open source module, ...).

## TODO

- Subtitle extraction and conversion (SRT, SAMI, VTT, ...)