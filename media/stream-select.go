package media

import (
	"sort"

	"github.com/tidwall/gjson"
)

// join-like function
func findStreamBestFit(
	codecType string,
	queryJSON gjson.Result,
	priorityInvertedIndex map[string]int,
	metaJSONArray []gjson.Result) int {

	// this function is my own "priority-aware sorting" algorithm

	type scoreType struct {
		index int
		score []int
	}

	scoreDimension := len(priorityInvertedIndex) + 1
	scoreBoard := make([]scoreType, len(metaJSONArray))

	queryKeys := flattenJSONKey(queryJSON)

	for i, streamMeta := range metaJSONArray {
		streamMetaKeys := flattenJSONKey(streamMeta)

		// alloc score section
		scoreBoard[i] = scoreType{
			index: int(streamMeta.Get("index").Int()),
			score: make([]int, scoreDimension)}

		// hash-join (match each line of flattened JSON metadata and JSON query)
		for queryKey := range queryKeys {
			if _, ok0 := streamMetaKeys[queryKey]; ok0 {
				if matchRegexPCRE2(queryJSON.Get(queryKey).String(), streamMeta.Get(queryKey).String()) {
					// if the queryKey is in "matching_priority"
					if scoreIndex, ok1 := priorityInvertedIndex[queryKey]; ok1 {
						scoreBoard[i].score[scoreIndex] += 1
					} else {
						scoreBoard[i].score[scoreDimension-1] += 1
					}
				}
			}
		}
	}

	sort.Slice(scoreBoard, func(i, j int) bool {
		// lower index in the score array == higher priority
		for s := 0; s < scoreDimension; s++ {
			if scoreBoard[i].score[s] == scoreBoard[j].score[s] {
				continue
			}
			return scoreBoard[i].score[s] > scoreBoard[j].score[s]
		}

		// if rank is not decided
		return scoreBoard[i].index < scoreBoard[j].index
	})

	return scoreBoard[0].index
}

func selectStream(arg commonArgType, mediaMetaJSON gjson.Result) map[string]transcodingInfoType {

	// group-by existing stream in the media file
	groupBy := make(map[string][]gjson.Result)
	for _, metaJSON := range mediaMetaJSON.Array() {
		codecType := metaJSON.Get("codec_type").String()
		if value, ok := groupBy[codecType]; ok {
			groupBy[codecType] = append(value, metaJSON)
		} else {
			groupBy[codecType] = []gjson.Result{metaJSON}
		}
	}

	result := make(map[string]transcodingInfoType)

	// intersect stream type between groupBy and queryInvertedIndex
	// i.e. stream=["Video":["video0", "video1"], "subtitle0"] AND query=["Video", "Audio"] = [best_fit(["video0", "video1"])]
	for codecType, metaJSONArray := range groupBy {
		{
			if _, ok := arg.queryJSON[codecType]; !ok {
				continue
			}
		}

		// after this line, the stream type is exists both input media side and query side
		temp := -1

		currentJSON := arg.queryJSON[codecType]
		queryJSON := currentJSON.Get("select_if")

		if len(metaJSONArray) > 1 && queryJSON.Exists() {
			// the input media have multiple stream of same type and the user specifies the stream information
			// it must pick best-fit single stream from input media stream

			// preprocessing for best-fit
			priorityInvertedIndex := make(map[string]int)

			priorityJSON := currentJSON.Get("select_priority")
			if priorityJSON.Exists() {
				for index, key := range priorityJSON.Array() {
					priorityInvertedIndex[key.String()] = index
				}
			}

			// find best-fit
			temp = findStreamBestFit(codecType, queryJSON, priorityInvertedIndex, metaJSONArray)
		} else {
			// the input media have single stream of same type or the user doesn't specifies the stream information
			temp = int(metaJSONArray[0].Get("index").Int())
		}
		result[codecType] =
			transcodingInfoType{
				streamIndex:         temp,
				isTranscodeRequired: true,
				streamInfo:          mediaMetaJSON.Array()[temp]}
	}

	return result
}
