package media

func decideStreamTranscodeRequired(arg *commonArgType) error {

	for codecType, target := range arg.info {
		// I'm consider that this stream require to transcode
		_isTranscodeRequired := true

		queryJSON := arg.queryJSON[codecType].Get("copy_if")

		if queryJSON.Exists() {
			queryKeys := flattenJSONKey(queryJSON)
			targetKeys := flattenJSONKey(target.streamInfo)

			// test all-match
			{
				allMatch := true
				for queryKey := range queryKeys {
					if _, ok0 := targetKeys[queryKey]; ok0 {
						if !matchRegexPCRE2(queryJSON.Get(queryKey).String(), target.streamInfo.Get(queryKey).String()) {
							// all-match failed
							allMatch = false
							break
						}
					}
				}

				// wow this stream passed an exam!
				if allMatch {
					// this stream doesn't need to transcode!
					_isTranscodeRequired = false
				}
			}
		}

		// apply result to original data
		arg.info[codecType] = transcodingInfoType{
			streamIndex:         target.streamIndex,
			isTranscodeRequired: _isTranscodeRequired,
			outFolder:           target.outFolder,
			outName:             target.outName,
			outExtension:        target.outExtension,
			streamInfo:          target.streamInfo,
		}
	}

	return nil
}
