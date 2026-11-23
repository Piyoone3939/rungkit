package profile

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
)

var (
	reDur     = regexp.MustCompile(`(?i)^(?:T#)?((?:\d+(?:\.\d+)?(?:ms|s|m|h|d)_?)+)$`)
	reDurPart = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)(ms|s|m|h|d)`)
)

// ParseDuration reads 500ms, 5s, 3.5s, 1m30s or an IEC literal like
// T#1s500ms and returns milliseconds (rounded).
func ParseDuration(s string) (int64, error) {
	m := reDur.FindStringSubmatch(s)
	if m == nil {
		return 0, i18n.Errorf("時間 %q が読めません", s)
	}
	var total float64
	for _, p := range reDurPart.FindAllStringSubmatch(m[1], -1) {
		n, _ := strconv.ParseFloat(p[1], 64)
		unit := map[string]float64{"ms": 1, "s": 1000, "m": 60000, "h": 3600000, "d": 86400000}[strings.ToLower(p[2])]
		total += n * unit
	}
	return int64(math.Round(total)), nil
}
