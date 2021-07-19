package media

import (
	"github.com/tidwall/gjson"
)

func _checkSanityValue(query gjson.Result, key string, valueType gjson.Type) {
	target := query.Get(key)
	switch {
	case !target.Exists():
		panic(`"` + key + `" does not exist`)
	case target.Type != valueType:
		panic(`"` + key + `" does not match type "` + valueType.String() + `", it is "` + target.Type.String() + `"`)
	}
}

func _checkSanityArray(query gjson.Result, key string, elementType gjson.Type) {
	target := query.Get(key)
	switch {
	case !target.Exists():
		panic(`"` + key + `" does not exist`)
	case !target.IsArray():
		panic(`"` + key + `" is not an array`)
	case target.IsArray() && len(target.Array()) == 0:
		panic(`"` + key + `" is an array but its length is 0`)
	case target.IsArray() && len(target.Array()) > 0:
		for _, value := range target.Array() {
			if value.Type != elementType {
				panic(`"` + key + `" contains wrong element type "` + value.Type.String() + `", it should be "` + elementType.String() + `"`)
			}
		}
	}
}

func _checkSanityMap(query gjson.Result, key string, elementType gjson.Type) {
	target := query.Get(key)
	for elemKey, elemValue := range target.Map() {
		if elemValue.Type != elementType {
			panic(`"` + elemKey + `" under "` + key + `" does not match the desired type "` + elementType.String() + `"`)
		}
	}
}

func CheckConfigSanity(queryJSON gjson.Result) error {
	var result error

	func() {
		defer func() {
			if e := recover(); e != nil {
				result = e.(error)
			}
		}()

		_checkSanityValue(queryJSON, "input", gjson.JSON)
		_checkSanityArray(queryJSON, "input.extension", gjson.String)

		_checkSanityValue(queryJSON, "output", gjson.JSON)
		_checkSanityArray(queryJSON, "output.stream", gjson.JSON)
		_checkSanityValue(queryJSON, "output.extension", gjson.String)

		for _, streamJSON := range queryJSON.Get("output.stream").Array() {
			_checkSanityValue(streamJSON, "codec_type", gjson.String)
			if queryJSON.Get("select_prefer").Exists() {
				_checkSanityValue(streamJSON, "select_prefer", gjson.JSON)
				if queryJSON.Get("select_priority").Exists() {
					_checkSanityValue(streamJSON, "select_priority", gjson.JSON)
				}
			} else {
				if queryJSON.Get("select_priority").Exists() {
					panic(`"select_priority" should not exist without "select_prefer"`)
				}
			}
			if queryJSON.Get("copy_if").Exists() {
				_checkSanityValue(streamJSON, "copy_if", gjson.JSON)
			}
			if queryJSON.Get("ffmpeg_parameter").Exists() {
				_checkSanityValue(streamJSON, "ffmpeg_parameter", gjson.JSON)
				_checkSanityMap(streamJSON, "ffmpeg_parameter", gjson.String)
			}

			_checkSanityValue(streamJSON, "temp_file_extension", gjson.String)
		}
	}()

	if result != nil {
		return result
	}

	return nil
}
