package filesys

import (
	"os"
	"path/filepath"
	"strings"
)

// PathBeautify ...
func PathBeautify(path string) string {
	absPath, e := filepath.Abs(path)
	if e != nil {
		panic(e)
	}

	if stat, e := os.Stat(absPath); os.IsNotExist(e) {
		panic(e)
	} else {

		if stat.IsDir() {
			return absPath + "/"
		} else {
			return absPath
		}
	}
}

// PathSplit ...
func PathSplit(path string) (string, string, string) {
	folder, name := filepath.Split(path)
	ext := filepath.Ext(path)
	name = strings.TrimSuffix(name, ext)

	return folder, name, ext
}
