package main

import (
	"image"
	"os"
	"reflect"
	"testing"
)

// TestExtractDeterministic regenerates a pose twice from the same sheet and
// requires identical pixels and anchors. Skips when the AI source sheet is
// not on this machine.
func TestExtractDeterministic(t *testing.T) {
	const src = "/home/rahul/Downloads/1.png"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("source sheet %s not present", src)
	}
	im, err := loadPNG(src)
	if err != nil {
		t.Fatal(err)
	}
	b := im.Bounds()
	bg := srcAt(im, b.Min.X+2, b.Min.Y+2)
	const factor = 16
	a := extractPose(im, image.Rect(0, 0, 600, 597), "wave", factor, bg, 10)
	c := extractPose(im, image.Rect(0, 0, 600, 597), "wave", factor, bg, 10)
	if a.w != c.w || a.h != c.h {
		t.Fatalf("dims differ: %dx%d vs %dx%d", a.w, a.h, c.w, c.h)
	}
	for i := range a.grid {
		if !reflect.DeepEqual(a.grid[i], c.grid[i]) {
			t.Fatal("grid not deterministic")
		}
	}
	if !reflect.DeepEqual(a.anch, c.anch) {
		t.Fatal("anchors not deterministic")
	}
}
