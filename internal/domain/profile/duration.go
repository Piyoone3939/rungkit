package profile

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reDur     = regexp.MustCompile(`(?i)^(?:T#)?((?:\d+(?:ms|s|m|h|d))+)$`)
	reDurPart = regexp.MustCompile(`(?i)(\d+)(ms|s|m|h|d)`)
)

// ParseDuration reads 500ms, 5s, 1m30s or an IEC literal like T#1s500ms and
// returns milliseconds.
func ParseDuration(s string) (int64, error) {
	m := reDur.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("時間 %q が読めません", s)
	}
	var total int64
	for _, p := range reDurPart.FindAllStringSubmatch(m[1], -1) {
		n, _ := strconv.ParseInt(p[1], 10, 64)
		unit := map[string]int64{"ms": 1, "s": 1000, "m": 60000, "h": 3600000, "d": 86400000}[strings.ToLower(p[2])]
		total += n * unit
	}
	return total, nil
}
