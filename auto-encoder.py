import os
import sys
import glob
import json
import subprocess
import termcolor 

import asyncio
import aiofiles
import time

def log(gpu, gpu_stream, infile, outfile, act, color):
    now = time.localtime()
    s = "%04d-%02d-%02d %02d:%02d:%02d" % (now.tm_year, now.tm_mon, now.tm_mday, now.tm_hour, now.tm_min, now.tm_sec)
    termcolor.cprint('[GPU ' + str(gpu) + '-' + str(gpu_stream) + '][' + s + '] ' + act + " '" + infile + "'" + ' -> ' + "'" + outfile + "'", color)


async def ffmpeg(infile, outfile, conf, gpu):
    arg = ['ffmpeg']
    arg += ['-hide_banner', '-loglevel', 'warning', '-y']
    arg += ['-threads', '0', '-thread_type', 'frame']
    arg += ['-analyzeduration', '2147483647','-probesize','2147483647']
    arg += ['-i', infile]
    arg += ['-threads', '0', '-max_muxing_queue_size', '1024']
    arg += ['-map', '0:v:' + conf['map']['video'], '-map', '0:a:' + conf['map']['audio']]

    # video
    if conf['video_encoding'] == 'yes':
        arg += ['-gpu', str(gpu)]
        for key, value in conf['v'].items():
            arg += ['-' + key + ':v', value]
    else:
        arg += ['-c:v','copy']

    # audio
    if conf['audio_encoding'] == 'yes':
        for key, value in conf['a'].items():
            arg += ['-' + key + ':a', value]
    else :
        arg += ['-c:a','copy']


    arg += [outfile]
    #print(''.join(i + ' ' for i in arg))

    proc = await asyncio.create_subprocess_exec(*arg, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE)
    await proc.wait()

    out, err = await proc.communicate()

    #print(f'[{arg!r} exited with {proc.returncode}]')
    if out:
        termcolor.cprint(f'[stdout]\n{out.decode()}\n','cyan')
    if err:
        termcolor.cprint(f'[stderr]\n{err.decode()}\n','yellow')
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

    # Then other files
    for ext in ['.mkv', '.avi']:
        for path in glob.glob(origin + '/**/*' + ext, recursive=True):
            infile = path
            outfile = os.path.join(os.path.dirname(path), os.path.basename(path).replace(ext, '.mp4'))
            conf = os.path.join(os.path.dirname(path), 'config.json')
            job = {'infile':infile, 'outfile':outfile, 'conf':conf}
            await q.put(job)

async def encoding(q, gpu, gpu_stream):
    while True:
        job = await q.get()

        infile = job['infile']
        outfile = job['outfile']
        try:
            async with aiofiles.open(job['conf'], mode='r') as f:
                text = await f.read()
                conf = json.loads(text)
        except:
            continue 

        try:
            log(gpu, gpu_stream, infile, outfile, 'START ', 'white')
            result = await ffmpeg(infile, outfile, conf, gpu)
        except:
            log(gpu, gpu_stream, infile, outfile, 'FAILED', 'red')
            q.task_done()
            continue

        log(gpu, gpu_stream, infile, outfile, 'FINISH', 'green')
        q.task_done()

async def main():
    q = asyncio.Queue()

    gpu0_0 = asyncio.ensure_future(encoding(q, 0, 0))
    gpu0_1 = asyncio.ensure_future(encoding(q, 0, 1))
    #gpu0_2 = asyncio.ensure_future(encoding(q, 0, 2))
    gpu1_0 = asyncio.ensure_future(encoding(q, 1, 0))
    gpu1_1 = asyncio.ensure_future(encoding(q, 1, 1))
    #gpu1_2 = asyncio.ensure_future(encoding(q, 1, 2))

    await producer(q)
    await q.join()

    gpu0_0.cancel()
    gpu0_1.cancel()
    gpu1_0.cancel()
    gpu1_1.cancel()

loop = asyncio.get_event_loop()
loop.run_until_complete(main())
loop.close()
