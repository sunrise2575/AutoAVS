package media

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/tidwall/gjson"
)

// prepare transcoding information
type transcodingInfoType struct {
	streamIndex                      int
	isTranscodeRequired              bool
	outFolder, outName, outExtension string
	streamInfo                       gjson.Result
}

func (this transcodingInfoType) outPath() string {
	return filepath.Join(this.outFolder, this.outName) + this.outExtension
}

func (this transcodingInfoType) Format(f fmt.State, c rune) {
	f.Write([]byte(fmt.Sprintf("(%v,%v,%v,%v,%v,[%v,%v])",
		this.streamIndex,
		this.isTranscodeRequired,
		this.outFolder,
		this.outName,
		this.outExtension,
		this.streamInfo.Get("codec_name").String(),
		this.streamInfo.Get("pix_fmt").String(),
	)))
}

type commonArgType struct {
	inPath     string
	mergedPath string
	queryJSON  map[string]gjson.Result
	info       map[string]transcodingInfoType
	mutex      *sync.Mutex
}
