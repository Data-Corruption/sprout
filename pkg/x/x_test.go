package x

import "testing"

func TestTernary(t *testing.T) {
	if got := Ternary(true, "yes", "no"); got != "yes" {
		t.Errorf("Ternary(true, ...) = %v; want yes", got)
	}
	if got := Ternary(false, "yes", "no"); got != "no" {
		t.Errorf("Ternary(false, ...) = %v; want no", got)
	}
	if got := Ternary(true, 1, 0); got != 1 {
		t.Errorf("Ternary(true, ...) = %v; want 1", got)
	}
}
