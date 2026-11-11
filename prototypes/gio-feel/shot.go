package main

import (
	"image"
	"image/png"
	"os"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// shot renders the prototype offscreen to a PNG, so it can be checked
// without opening a window: go run . -shot out.png
func shot(path string, u *ui) error {
	size := image.Pt(1500, 940)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		return err
	}
	defer win.Release()

	u.covers = newCoverCache(6, func() {})
	u.play(min(40, len(u.tracks)-1))
	u.selAlbum = u.tracks[u.playing].albumIdx
	var ops op.Ops
	now := time.Now()
	for i := range 40 { // let covers decode and fades settle
		ops.Reset()
		now = now.Add(50 * time.Millisecond)
		gtx := layout.Context{Ops: &ops, Now: now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(size)}
		u.layout(gtx)
		if err := win.Frame(&ops); err != nil {
			return err
		}
		if i < 30 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := win.Screenshot(img); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
