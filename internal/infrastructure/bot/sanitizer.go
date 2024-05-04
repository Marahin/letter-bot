package bot

import "strings"

// sanitizeTimeFormat forgives user for their weird time formats
// that may be incompatible with go's formats.
func sanitizeTimeFormat(startAt string) string {
	startAtStr := strings.ReplaceAll(startAt, ".", ":")       // 19.00 -> 19:00
	startAtStr = strings.ReplaceAll(startAtStr, ";", ":")     // 19;00 -> 19:00
	startAtStr = strings.ReplaceAll(startAtStr, "24:", "00:") // 24:00 -> 00:00

	return startAtStr
}
