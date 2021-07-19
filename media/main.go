package media

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/sunrise2575/AutoAVS/filesys"
	"github.com/tidwall/gjson"
)

func setTempFilePath(arg *commonArgType) error {
	originFolder, originName, _ := filesys.PathSplit(arg.inPath)
	if !filesys.IsDir(originFolder) {
		return fmt.Errorf("%v is not a normal folder", originFolder)
	}

	for codecType, _info := range arg.info {
		arg.info[codecType] =
			transcodingInfoType{
				streamIndex:         _info.streamIndex,
				isTranscodeRequired: _info.isTranscodeRequired,
				outFolder:           originFolder,
				outName:             "." + originName + "__transcoding",
				outExtension:        "." + arg.queryJSON[codecType].Get("temp_file_extension").String(),
				streamInfo:          _info.streamInfo,
			}
	}

	return nil
}

func Transcode(inPath string, configJSON gjson.Result, gpuID int) error {

	var e error
	if inPath, e = filepath.Abs(inPath); e != nil {
		return e
	}

	// get the file's media metadata
	mediaMetaJSON := gjson.Get(getMetadataFromFFProbe(inPath), "streams")

	var arg commonArgType
	arg.inPath = inPath
	arg.mergedPath = ""
	arg.queryJSON = makeQueryInvertedIndex(configJSON)
	arg.info = selectStream(arg, mediaMetaJSON)
	arg.mutex = &sync.Mutex{}

	fmt.Println(arg.info)

	if e = decideStreamTranscodeRequired(&arg); e != nil {
		return e
	}

	if e = setTempFilePath(&arg); e != nil {
		return e
	}

	// prepare channel
	{
		transcodingStatus := make(map[string]error)

		for codecType := range arg.info {
			transcodingStatus[codecType] = nil
		}

		var wg sync.WaitGroup

		// exploit parallel job
		for codecType := range transcodingStatus {
			wg.Add(1)
			go func(codecType string) {
				defer wg.Done()
				transcodingStatus[codecType] = runFFMpegTranscode(arg, codecType, gpuID)
			}(codecType)
		}

		wg.Wait()

		for _, e := range transcodingStatus {
			if e != nil {
				// TODO: rollback (remove temp files)
				return e
			}
		}
	}

	if e := runMerge(&arg, configJSON); e != nil {
		// TODO: rollback (remove temp files)
		return e
	}
	if e := flushFiles(arg, configJSON); e != nil {
		// TODO: rollback
		return e
	}

	return nil
}
