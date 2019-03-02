import os
import sys
import glob
import json
import subprocess
import termcolor 

import asyncio
import aiofiles
import time

def log(gpu, gpu_stream, path, act, color):
    now = time.localtime()
    s = "%04d-%02d-%02d %02d:%02d:%02d" % (now.tm_year, now.tm_mon, now.tm_mday, now.tm_hour, now.tm_min, now.tm_sec)
    termcolor.cprint('[GPU ' + str(gpu) + '-' + str(gpu_stream) + '][' + s + '] ' + act + " '" + path + "'", color)

async def ffmpeg(path, ext, conf, gpu):
    #filename
    infile = path
    outfile = os.path.join(os.path.dirname(path), os.path.basename(path).replace(ext, '.mp4'))

    #arguments
    arg = ['ffmpeg']
    arg += ['-hide_banner', '-loglevel', 'error', '-y']
    arg += ['-threads', '0', '-thread_type', 'frame']
    arg += ['-analyzeduration', '2147483647','-probesize','2147483647']
    arg += ['-i', infile]
    arg += ['-threads', '0', '-max_muxing_queue_size', '1024']
    arg += ['-map', '0:v:' + conf['map']['video'], '-map', '0:a:' + conf['map']['audio']]

    # video setting
    if conf['video_encoding'] == 'yes':
        arg += ['-gpu', str(gpu)]
        for key, value in conf['v'].items():
            arg += ['-' + key + ':v', value]
    else:
        arg += ['-c:v','copy']

    # audio setting
    if conf['audio_encoding'] == 'yes':
        for key, value in conf['a'].items():
            arg += ['-' + key + ':a', value]
    else :
        arg += ['-c:a','copy']

    arg += [outfile]

    # encoding
    proc = await asyncio.create_subprocess_exec(*arg, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE)
    await proc.wait()

    out, err = await proc.communicate()

    if out:
        termcolor.cprint(f'[stdout]\n{out.decode()}\n','cyan')
    if err:
        termcolor.cprint(f'[stderr]\n{err.decode()}\n','yellow')
        raise 1

async def ffmpeg_split(path, ext, conf, gpu):
    if conf['video_encoding'] != 'yes' or conf['audio_encoding'] != 'yes':
        raise 1

    # filename
    infile = path
    outfile_v = os.path.join(os.path.dirname(path), os.path.basename(path).replace(ext, '.mp4'))
    outfile_a = os.path.join(os.path.dirname(path), os.path.basename(path).replace(ext, '.aac'))
    outfile_m = os.path.join(os.path.dirname(path), os.path.basename(path).replace(ext, ' merged.mp4'))

    # common arguments 
    arg_c = ['ffmpeg']
    arg_c += ['-hide_banner', '-loglevel', 'error', '-y']
    arg_c += ['-threads', '0', '-thread_type', 'frame']
    arg_c += ['-analyzeduration', '2147483647','-probesize','2147483647']
    arg_c += ['-i', infile]
    arg_c += ['-threads', '0', '-max_muxing_queue_size', '1024']

    # video arguments
    arg_v = []
    arg_v += arg_c
    arg_v += ['-gpu', str(gpu)]
    arg_v += ['-map', '0:v:' + conf['map']['video'], '-an']
    for key, value in conf['v'].items():
        arg_v += ['-' + key + ':v', value]

    arg_v += [outfile_v]

    # audio arguments
    arg_a = []
    arg_a += arg_c
    arg_a += ['-vn', '-map', '0:a:' + conf['map']['audio']]
    for key, value in conf['a'].items():
        arg_a += ['-' + key + ':a', value]
        
    arg_a += [outfile_a]

    #print(''.join(i + ' ' for i in arg_v))
    #print(''.join(i + ' ' for i in arg_a))

    def _remove_intermediate(_v, _a):
        if os.path.exists(_v) == True:
            os.remove(_v)
        if os.path.exists(_a) == True:
            os.remove(_a)

    # split encoding
    proc_v = await asyncio.create_subprocess_exec(*arg_v, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE)
    proc_a = await asyncio.create_subprocess_exec(*arg_a, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE)

    out_v, err_v = await proc_v.communicate()
    out_a, err_a = await proc_a.communicate()

    await proc_v.wait()
    await proc_a.wait()

    if out_v:
        termcolor.cprint(f'[stdout_video]\n{out_v.decode()}\n','cyan')
    if err_v:
        _remove_intermediate(outfile_v, outfile_a)
        termcolor.cprint(f'[stdout_video]\n{err_v.decode()}\n','yellow')
        raise 1

    if out_a:
        termcolor.cprint(f'[stdout_audio]\n{out_a.decode()}\n','cyan')
    if err_a:
        _remove_intermediate(outfile_v, outfile_a)
        termcolor.cprint(f'[stderr_audio]\n{err_a.decode()}\n','yellow')
        raise 1

    # merge
    arg_m = ['ffmpeg']
    arg_m += ['-i',outfile_v]
    arg_m += ['-i',outfile_a]
    arg_m += ['-hide_banner', '-loglevel', 'error', '-y']
    arg_m += ['-c:v','copy','-c:a','copy']
    arg_m += [outfile_m]

    #print(''.join(i + ' ' for i in arg_m))
    proc_m = await asyncio.create_subprocess_exec(*arg_m, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE)
    out_m, err_m = await proc_m.communicate()
    await proc_m.wait()

    if out_m:
        termcolor.cprint(f'[stdout_merge]\n{out_m.decode()}\n','cyan')
    if err_m:
        _remove_intermediate(outfile_v, outfile_a)
        termcolor.cprint(f'[stderr_merge]\n{err_m.decode()}\n','yellow')
        raise 1

    # delete intermediate file
    _remove_intermediate(outfile_v, outfile_a)

    # rename final file
    if os.path.exists(outfile_m) == True:
        os.rename(outfile_m, outfile_v)
    else:
        raise 1

async def producer(q):
    origin = sys.argv[1]

    # MP4 first
    '''
    for path in glob.glob(origin + '/**/*.mp4', recursive=True):
        infile = path
        outfile = os.path.join(os.path.dirname(path), os.path.basename(path).replace('.mp4', '_new.mp4'))
        conf = os.path.join(os.path.dirname(path), 'config.json')
        job = {'infile':infile, 'outfile':outfile, 'conf':conf}
        await q.put(job)
    '''

    for ext in ['.mkv', '.avi']:
        for path in glob.glob(origin + '/**/*' + ext, recursive=True):
            conf = os.path.join(os.path.dirname(path), 'config.json')
            job = {'path':path, 'ext':ext, 'conf':conf}
            await q.put(job)

async def encoding(q, gpu, gpu_stream):
    while True:
        job = await q.get()

        path = job['path']
        ext = job['ext']
        conf = []

        # open config file
        try:
            async with aiofiles.open(job['conf'], mode='r') as f:
                text = await f.read()
                conf = json.loads(text)
        except:
            continue 

        # start split encoding
        try:
            log(gpu, gpu_stream, path, 'START  ', 'white')
            await ffmpeg_split(path, ext, conf, gpu)
        except:
            # Try original encoding
            try:
                log(gpu, gpu_stream, path, 'RESTART', 'cyan')
                await ffmpeg(path, ext, conf, gpu)
            except:
                # on failure
                log(gpu, gpu_stream, path, 'FAILED ', 'red')
                q.task_done()
                continue

        log(gpu, gpu_stream, path, 'FINISH ', 'green')
        q.task_done()

async def main():
    q = asyncio.Queue()

    gpu0_0 = asyncio.ensure_future(encoding(q, 0, 0))
    gpu1_0 = asyncio.ensure_future(encoding(q, 1, 0))
    gpu0_1 = asyncio.ensure_future(encoding(q, 0, 1))
    gpu1_1 = asyncio.ensure_future(encoding(q, 1, 1))

    await producer(q)
    await q.join()

    gpu0_0.cancel()
    gpu0_1.cancel()
    gpu1_0.cancel()
    gpu1_1.cancel()

loop = asyncio.get_event_loop()
loop.run_until_complete(main())
loop.close()
