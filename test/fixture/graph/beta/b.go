package beta

import "example.com/fixture/graph/gamma"

func Process(v int) int {
	switch {
	case v > 100:
		return gamma.Normalize(v) + 1
	case v > 50:
		return gamma.Normalize(v)
	}
	return gamma.Normalize(v - 1)
}
