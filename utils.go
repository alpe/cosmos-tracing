package tracing

import "strings"

var (
	// MaxStoreTraced max size. Spans are lost when the log size is too big
	MaxStoreTraced    = 5_000
	MaxSDKMsgTraced   = 5_000
	MaxSDKLogTraced   = 5_000
	MaxIBCPacketDescr = 5_000
	DefaultMaxLength  = 10_000
)

func cutLength(storeData string, max int) string {
	if len(storeData) > max {
		storeData = storeData[0:max] + "... >-8 cut"
	}
	return strings.TrimSpace(storeData)
}
