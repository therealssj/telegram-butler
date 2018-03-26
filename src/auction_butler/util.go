package auction_butler

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

func niceDuration(d time.Duration) string {
	if d < 0 {
		return d.String()
	}

	var hours, minutes, seconds int
	seconds = int(d.Seconds())
	hours, seconds = seconds/3600, seconds%3600
	minutes, seconds = seconds/60, seconds%60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		} else {
			return fmt.Sprintf("%dh", hours)
		}
	} else {
		if minutes > 0 {
			if seconds > 0 {
				return fmt.Sprintf("%dm%ds", minutes, seconds)
			} else {
				return fmt.Sprintf("%dm", minutes)
			}
		} else {
			return fmt.Sprintf("%ds", seconds)
		}
	}
}

func appendField(fields []string, name, format string, args ...interface{}) []string {
	value := fmt.Sprintf(format, args...)
	return append(fields, fmt.Sprintf("*%s*: %s", strings.Title(name), value))
}


func parseDuration(args string) (time.Duration, error) {
	hours, err := strconv.ParseFloat(args, 64)
	if err == nil {
		return time.Second * time.Duration(hours*3600), nil
	}

	return time.ParseDuration(args)
}

func SplitToString(a []int, sep string) string {
	if len(a) == 0 {
		return ""
	}

	b := make([]string, len(a))
	for i, v := range a {
		b[i] = strconv.Itoa(v)
	}
	return strings.Join(b, sep)
}
