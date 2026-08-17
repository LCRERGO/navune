package dup

import "testing"

func TestUnique(t *testing.T) {
	got := unique([]int{1, 2, 2, 3})
	if len(got) != 3 {
		t.Fatalf("want 3 got %d", len(got))
	}
}
