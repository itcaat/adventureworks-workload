package app

import (
	"database/sql"
	"fmt"
)

type TrafficStats struct {
	Sent     int64
	Received int64
}

func requestBytes(query string, args ...any) int64 {
	total := int64(len(query))
	for _, arg := range args {
		total += paramBytes(arg)
	}
	return total
}

func paramBytes(v any) int64 {
	switch x := v.(type) {
	case sql.NamedArg:
		return int64(len(x.Name)) + valueBytes(x.Value)
	default:
		return valueBytes(v)
	}
}

func valueBytes(v any) int64 {
	switch x := v.(type) {
	case string:
		return int64(len(x))
	case []byte:
		return int64(len(x))
	case int:
		return 8
	case int32:
		return 4
	case int64:
		return 8
	case float64:
		return 8
	case bool:
		return 1
	case nil:
		return 0
	default:
		return 8
	}
}

func FormatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
