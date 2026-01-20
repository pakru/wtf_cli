package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestAddOverlayLayerCentersOddSizes(t *testing.T) {
	layers := addOverlayLayer(nil, "abc", 9, 5, 2)
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}
	layer := layers[0]
	if layer.GetX() != 3 || layer.GetY() != 2 {
		t.Fatalf("unexpected position: x=%d y=%d", layer.GetX(), layer.GetY())
	}
	if layer.GetWidth() != 3 || layer.GetHeight() != 1 {
		t.Fatalf("unexpected size: w=%d h=%d", layer.GetWidth(), layer.GetHeight())
	}
	if layer.GetZ() != 2 {
		t.Fatalf("unexpected z-index: %d", layer.GetZ())
	}
}

func TestAddOverlayLayerANSIAndUnicodeWidth(t *testing.T) {
	view := "\x1b[31m界界\x1b[0m"
	layers := addOverlayLayer(nil, view, 10, 4, 1)
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}
	layer := layers[0]
	if layer.GetWidth() != 4 || layer.GetHeight() != 1 {
		t.Fatalf("unexpected size: w=%d h=%d", layer.GetWidth(), layer.GetHeight())
	}
	if layer.GetX() != 3 || layer.GetY() != 1 {
		t.Fatalf("unexpected position: x=%d y=%d", layer.GetX(), layer.GetY())
	}
}

func TestAddOverlayLayerClampsToScreen(t *testing.T) {
	view := strings.Repeat("x", 20)
	layers := addOverlayLayer(nil, view, 8, 3, 1)
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}
	layer := layers[0]
	if layer.GetWidth() != 8 || layer.GetHeight() != 1 {
		t.Fatalf("unexpected size: w=%d h=%d", layer.GetWidth(), layer.GetHeight())
	}
	if layer.GetX() != 0 || layer.GetY() != 1 {
		t.Fatalf("unexpected position: x=%d y=%d", layer.GetX(), layer.GetY())
	}
}

func TestOverlayLayersRespectZOrder(t *testing.T) {
	layers := []*lipgloss.Layer{
		lipgloss.NewLayer("base").Z(0),
	}
	layers = addOverlayLayer(layers, "under", 5, 1, 1)
	layers = addOverlayLayer(layers, "over", 5, 1, 2)

	canvas := lipgloss.NewCanvas(layers...)
	out := canvas.Render()

	if !strings.Contains(out, "over") {
		t.Fatalf("expected top overlay content, got %q", out)
	}
	if strings.Contains(out, "under") {
		t.Fatalf("expected lower overlay to be hidden, got %q", out)
	}
}
