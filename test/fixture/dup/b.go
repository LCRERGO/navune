package dup

type vector struct {
	dx, dy int
}

func distanceB(v vector) int {
	dx := v.dx * v.dx
	dy := v.dy * v.dy
	sum := dx + dy
	if sum < 0 {
		return -sum
	}
	return sum
}

func magnitudeB(vals []int) int {
	total := 0
	for _, v := range vals {
		if v > 0 {
			total += v
		}
	}
	return total
}

func unique(vals []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, v := range vals {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
