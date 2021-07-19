package media

import (
	"os/exec"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/sunrise2575/AutoAVS/filesys"
	"github.com/tidwall/gjson"
)

func getMetadataFromFFProbe(filePath string) string {
	if !filesys.IsFile(filePath) {
		return ""
	}

	// get info
	result, e := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", filePath).Output()
	if e != nil {
		return ""
	}

	return string(result)
}

func flattenJSONKey(json gjson.Result) map[string]struct{} {
	root := "@this"
	reserve := []string{root}
	complete := make(map[string]struct{})

	//fmt.Printf("%v -> %v -> %v INIT\n", reserve, nil, complete)
	//fmt.Println(json)

	for {
		// pop
		current := reserve[0]
		reserve = reserve[1:]

		//fmt.Printf("%v -> %v -> %v PRE\n", reserve, current, complete)

		for k, v := range json.Get(current).Map() {
			newkey := strings.TrimPrefix(current+"."+k, "@this.")
			if v.Type == gjson.JSON {
				// push
				reserve = append(reserve, newkey)
				//fmt.Printf("%v -> %v -> %v JSON\n", reserve, current, complete)
			} else {
				complete[newkey] = struct{}{}
				//fmt.Printf("%v -> %v -> %v ELSE\n", reserve, current, complete)
			}
		}

		if len(reserve) == 0 {
			break
		}
	}

	return complete
}

func matchRegexPCRE2(regex string, target string) bool {
	re, _ := regexp2.Compile(regex, 0)
	matched, _ := re.MatchString(target)
	return matched
}

func timestamp() string {
	timeFormat := "2006-01-02T150405Z0700"
	return "(" + time.Now().Format(timeFormat) + ")"
}

func makeQueryInvertedIndex(configJSON gjson.Result) map[string]gjson.Result {
	result := make(map[string]gjson.Result)
	for _, queryJSON := range configJSON.Get("output.stream").Array() {
		result[queryJSON.Get("codec_type").String()] = queryJSON
	}

	return result
}
