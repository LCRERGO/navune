// Package dup holds duplicated helper functions (copy-paste).
package dup

type point struct {
	x, y int
}

func distanceA(p point) int {
	dx := p.x * p.x
	dy := p.y * p.y
	sum := dx + dy
	if sum < 0 {
		return -sum
	}
	return sum
}

func magnitudeA(vals []int) int {
	total := 0
	for _, v := range vals {
		if v > 0 {
			total += v
		}
	}
	return total
}
