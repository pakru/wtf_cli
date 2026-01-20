package render

import "testing"

func TestCenterRectOddSizes(t *testing.T) {
	x, y, w, h := CenterRect(3, 1, 9, 5)
	if x != 3 || y != 2 || w != 3 || h != 1 {
		t.Fatalf("unexpected rect: x=%d y=%d w=%d h=%d", x, y, w, h)
	}
}

func TestCenterRectClampsToScreen(t *testing.T) {
	x, y, w, h := CenterRect(10, 7, 6, 4)
	if x != 0 || y != 0 || w != 6 || h != 4 {
		t.Fatalf("unexpected rect: x=%d y=%d w=%d h=%d", x, y, w, h)
	}
}

func TestClampRectNegativeOrigin(t *testing.T) {
	x, y, w, h := ClampRect(-2, -1, 5, 4, 4, 3)
	if x != 0 || y != 0 || w != 4 || h != 3 {
		t.Fatalf("unexpected rect: x=%d y=%d w=%d h=%d", x, y, w, h)
	}
}

func TestViewportHeight(t *testing.T) {
	if ViewportHeight(0) != 0 {
		t.Fatal("expected height 0 for screen height 0")
	}
	if ViewportHeight(1) != 0 {
		t.Fatal("expected height 0 for screen height 1")
	}
	if ViewportHeight(5) != 4 {
		t.Fatalf("expected height 4 for screen height 5, got %d", ViewportHeight(5))
	}
}
