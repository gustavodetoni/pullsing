package evaluation

import "github.com/gustavodetoni/pullsing/sdk/go/types"

func Enabled(snapshot types.Snapshot, key string) bool {
	flag, ok := snapshot.Flags[key]
	if !ok {
		return false
	}

	return flag.Enabled && flag.Value
}
