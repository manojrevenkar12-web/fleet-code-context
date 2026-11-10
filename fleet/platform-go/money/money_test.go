package money

import "testing"

func TestApplyBasisPoints(t *testing.T) {
	if got := ApplyBasisPoints(1000, 250); got != 25 {
		t.Fatalf("got %d", got)
	}
}
