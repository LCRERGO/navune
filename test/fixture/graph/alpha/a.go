// Package alpha forms a cycle with beta and gamma.
package alpha

import (
	"example.com/fixture/graph/beta"
	"example.com/fixture/graph/gamma"
)

type Service struct{}

func (s *Service) Handle(x int) int {
	if x > 10 {
		return beta.Process(x)
	}
	return gamma.Normalize(x)
}

func Helper(p int) int {
	if p < 0 {
		return -p
	}
	if p == 0 {
		return 0
	}
	sum := 0
	for i := 0; i < p; i++ {
		if i%2 == 0 {
			sum += i
		}
	}
	return sum
}
