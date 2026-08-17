package gamma

import (
	"example.com/fixture/graph/alpha"
	"example.com/fixture/graph/beta"
)

type Config struct {
	Threshold int
}

func Normalize(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func Load() *Config {
	a := &alpha.Service{}
	_ = a
	beta.Process(1)
	return &Config{Threshold: 10}
}
