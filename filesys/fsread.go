package filesys

import (
	"os"
)

// IsFile checks that the path is a file
func IsFile(path string) bool {
	fileStat, e := os.Stat(path)

	if os.IsNotExist(e) || fileStat.IsDir() {
		return false
	}

	return true
}

// IsDir checks that the path is a directory
func IsDir(path string) bool {
	fileStat, e := os.Stat(path)

	if os.IsNotExist(e) || !fileStat.IsDir() {
		return false
	}

	return true
}
