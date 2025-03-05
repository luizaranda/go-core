package telemetry

import (
	"fmt"
	"strings"
)

var _patternReplacer = strings.NewReplacer(
	"{", "_",
	"}", "",
)

// SanitizeMetricTagValue sanitizes the given value in a standard way. It:
//   - Trims suffix "/".
//   - Replace "{" with "_"
//   - Remove  "}".
func SanitizeMetricTagValue(value string) string {
	if value == "" {
		return ""
	}

	value = strings.TrimRight(value, "/")
	if value == "" {
		return "/"
	}

	return _patternReplacer.Replace(value)
}

// Tags will add a tag:value pair to the list of tags for a metric.
// This func will panic if number if arguments is odd, any tag is not a string or any value is not
// of one of the supported types (string, stringer, all integer types and bool).
func Tags(nameValue ...interface{}) []string {
	if len(nameValue)%2 != 0 {
		panic("number of arguments must be even")
	}

	tags := make([]string, 0, len(nameValue)/2)
	for i := 0; i+1 < len(nameValue); i += 2 {
		tags = append(tags, fmt.Sprintf("%s:%s", nameValue[i].(string), stringerize(nameValue[i+1])))
	}

	return tags
}

func stringerize(value interface{}) string {
	switch t := value.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, bool:
		return fmt.Sprintf("%v", value)
	default:
		panic(fmt.Sprintf("type %T is unsupported", value))
	}
}
