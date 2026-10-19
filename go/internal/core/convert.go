package core

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// ToString is JavaScript's String(v) for a configuration value.
func ToString(v Value) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return js.NumberToString(x)
	case string:
		return x
	case []Value:
		parts := make([]string, len(x))
		for i, e := range x {
			if e != nil {
				parts[i] = ToString(e)
			}
		}
		return strings.Join(parts, ",")
	}
	return "[object Object]"
}

// ToNumber is JavaScript's Number(v) for a configuration value.
func ToNumber(v Value) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case bool:
		if x {
			return 1
		}
		return 0
	case float64:
		return x
	case string:
		return js.ToNumber(x)
	case []Value:
		return js.ToNumber(ToString(x))
	}
	return math.NaN()
}

// IsRecord reports whether v is a plain object, not an array or null.
func IsRecord(v Value) bool {
	_, ok := v.(*Object)
	return ok
}
