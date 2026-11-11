// PROTOTYPE — throwaway, never merged (branch proto/gio-feel).
//
// Question it answers: can a Go GUI built on Gio feel as fluid and look as
// good as GPUI (Zed, herdr-app)? One Zed-styled screen over the real waves
// library: a virtualized track list, an album grid with lazily decoded
// covers, a player bar with a progress bar animated every frame.
//
// Keys: j/k ↑/↓ move · h/l ←/→ move in the grid · ctrl+d/ctrl+u half page ·
// g/G top/bottom · tab switch pane · enter play · space pause · f frame stats.
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

// Zed "One Dark" tokens.
var (
	cEditorBg   = rgb(0x282c33)
	cPanelBg    = rgb(0x2f343e)
	cTitleBar   = rgb(0x3b414d)
	cBorder     = rgb(0x464b57)
	cBorderSoft = rgb(0x363c46)
	cText       = rgb(0xdce0e5)
	cMuted      = rgb(0xa9afbc)
	cDim        = rgb(0x878a98)
	cAccent     = rgb(0x74ade8)
	cHover      = rgb(0x363c46)
	cSelected   = rgb(0x454a56)
	cSelUnfocus = rgb(0x3a3f4a)
)

const (
	sans = "IBM Plex Sans"
	mono = "IBM Plex Mono"
)

type pane int

const (
	paneTracks pane = iota
	paneAlbums
)

type rowState struct {
	btn   widget.Clickable
	hover float32
	seen  int
}

type ui struct {
	th     *material.Theme
	tracks []track
	albums []album
	source string
	covers *coverCache

	focus      pane
	selTrack   int
	selAlbum   int
	trackList  widget.List
	albumList  widget.List
	trackRows  map[int]*rowState
	albumCells map[int]*rowState
	fade       map[int]float32 // album idx → cover fade-in progress
	cols       int
	frame      int
	lastFrame  time.Time
	dt         time.Duration // since the previous frame, 0 after an idle gap

	playing int // track idx, -1 when nothing plays
	paused  bool
	elapsed time.Duration

	showStats  bool
	fps        float64
	layoutCost time.Duration
	startup    time.Duration
	loadTime   time.Duration
}

var processStart = time.Now()

func main() {
	tracks, albums, source := loadLibrary()
	loadTime := time.Since(processStart)

	u := &ui{
		th: newTheme(), tracks: tracks, albums: albums, source: source, loadTime: loadTime,
		trackRows: map[int]*rowState{}, albumCells: map[int]*rowState{}, fade: map[int]float32{},
		playing: -1, showStats: true, cols: 1,
	}
	u.trackList.Axis = layout.Vertical
	u.albumList.Axis = layout.Vertical

	if len(os.Args) == 3 && os.Args[1] == "-shot" {
		if err := shot(os.Args[2], u); err != nil {
			log.Fatal(err)
		}
		return
	}

	go func() {
		w := new(app.Window)
		w.Option(app.Title("waves · Gio prototype"), app.Size(unit.Dp(1500), unit.Dp(940)))
		u.covers = newCoverCache(6, w.Invalidate)
		if err := run(w, u); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window, u *ui) error {
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			start := time.Now()
			u.layout(gtx)
			u.record(time.Since(start))
			e.Frame(gtx.Ops)
		}
	}
}

func newTheme() *material.Theme {
	th := material.NewTheme()
	var faces []font.FontFace
	dir := os.Getenv("PROTO_FONT_DIR")
	for _, f := range []string{"IBMPlexSans-Regular.ttf", "IBMPlexSans-Medium.ttf", "IBMPlexSans-SemiBold.ttf", "IBMPlexMono-Regular.ttf"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		if c, err := opentype.ParseCollection(b); err == nil {
			faces = append(faces, c...)
		}
	}
	if len(faces) == 0 {
		fmt.Fprintln(os.Stderr, "prototype: PROTO_FONT_DIR has no IBM Plex, falling back to Go fonts")
	}
	th.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(append(faces, gofont.Collection()...)))
	th.Face = sans
	th.Palette = material.Palette{Bg: cEditorBg, Fg: cText, ContrastBg: cAccent, ContrastFg: cEditorBg}
	return th
}

// record keeps moving averages of the frame rate (only meaningful while
// something animates: Gio draws nothing when idle) and of the CPU layout cost.
func (u *ui) record(cost time.Duration) {
	if u.dt > 0 {
		u.fps = ema(u.fps, 1/u.dt.Seconds())
	}
	u.layoutCost = time.Duration(ema(float64(u.layoutCost), float64(cost)))
	if u.startup == 0 {
		u.startup = time.Since(processStart)
	}
}

func ema(prev, v float64) float64 {
	if prev == 0 {
		return v
	}
	return prev*0.9 + v*0.1
}

func (u *ui) layout(gtx C) {
	u.dt = gtx.Now.Sub(u.lastFrame)
	if u.lastFrame.IsZero() || u.dt > 250*time.Millisecond {
		u.dt = 0
	}
	u.lastFrame = gtx.Now
	u.frame++
	u.handleKeys(gtx)

	if u.playing >= 0 && !u.paused {
		u.elapsed += u.dt
		if u.elapsed >= duration(u.playing) {
			u.elapsed = 0
			u.playing = (u.playing + 1) % len(u.tracks)
		}
		gtx.Execute(op.InvalidateCmd{})
	}

	paint.Fill(gtx.Ops, cEditorBg)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.titleBar),
		layout.Flexed(1, func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(0.56, func(gtx C) D { return u.pane(gtx, paneTracks, "Tracks", len(u.tracks), u.trackPane) }),
				layout.Rigid(vline),
				layout.Flexed(0.44, func(gtx C) D { return u.pane(gtx, paneAlbums, "Albums", len(u.albums), u.albumPane) }),
			)
		}),
		layout.Rigid(hline),
		layout.Rigid(u.playerBar),
	)

	for i, rs := range u.trackRows {
		if rs.seen != u.frame {
			delete(u.trackRows, i)
		}
	}
	for i, rs := range u.albumCells {
		if rs.seen != u.frame {
			delete(u.albumCells, i)
		}
	}
}

// --- keyboard ---------------------------------------------------------------

func (u *ui) handleKeys(gtx C) {
	filters := []event.Filter{
		key.Filter{Name: "J"}, key.Filter{Name: "K"}, key.Filter{Name: "H"}, key.Filter{Name: "L"}, key.Filter{Name: "F"},
		key.Filter{Name: key.NameDownArrow}, key.Filter{Name: key.NameUpArrow}, key.Filter{Name: key.NameLeftArrow}, key.Filter{Name: key.NameRightArrow},
		key.Filter{Name: "D", Required: key.ModCtrl}, key.Filter{Name: "U", Required: key.ModCtrl},
		key.Filter{Name: "G", Optional: key.ModShift}, key.Filter{Name: key.NameTab, Optional: key.ModShift},
		key.Filter{Name: key.NameReturn}, key.Filter{Name: key.NameEnter}, key.Filter{Name: key.NameSpace},
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		u.onKey(e)
	}
}

func (u *ui) onKey(e key.Event) {
	step, page := 1, 1
	sel, n, list := &u.selTrack, len(u.tracks), &u.trackList.List
	if u.focus == paneAlbums {
		step = u.cols
		sel, n, list = &u.selAlbum, len(u.albums), &u.albumList.List
	}
	page = max(1, list.Position.Count/2) * step

	switch e.Name {
	case "J", key.NameDownArrow:
		*sel += step
	case "K", key.NameUpArrow:
		*sel -= step
	case "L", key.NameRightArrow:
		if u.focus == paneAlbums {
			*sel++
		}
	case "H", key.NameLeftArrow:
		if u.focus == paneAlbums {
			*sel--
		}
	case "D":
		*sel += page
	case "U":
		*sel -= page
	case "G":
		if e.Modifiers.Contain(key.ModShift) {
			*sel = n - 1
		} else {
			*sel = 0
		}
	case key.NameTab:
		u.focus = 1 - u.focus
		return
	case key.NameReturn, key.NameEnter:
		if u.focus == paneTracks {
			u.play(u.selTrack)
		} else {
			u.playAlbum(u.selAlbum)
		}
		return
	case key.NameSpace:
		u.paused = !u.paused
		return
	case "F":
		u.showStats = !u.showStats
		return
	}
	*sel = max(0, min(n-1, *sel))
	row := *sel / step
	ensureVisible(list, row)
}

func ensureVisible(l *layout.List, row int) {
	p := l.Position
	switch {
	case row < p.First || (row == p.First && p.Offset > 0):
		l.ScrollTo(row)
	case p.Count > 1 && row >= p.First+p.Count-1:
		l.ScrollTo(max(0, row-p.Count+2))
	}
}

func (u *ui) play(i int) {
	u.playing, u.paused, u.elapsed = i, false, 0
}

func (u *ui) playAlbum(a int) {
	for i, t := range u.tracks {
		if t.albumIdx == a {
			u.play(i)
			return
		}
	}
}

func duration(i int) time.Duration {
	return time.Duration(150+(i*7919)%210) * time.Second
}

// --- chrome -----------------------------------------------------------------

func (u *ui) titleBar(gtx C) D {
	h := gtx.Dp(38)
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, h))
	paint.FillShape(gtx.Ops, cTitleBar, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return layout.Inset{Left: 14, Right: 14}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return u.text(gtx, "waves", 14, cText, font.SemiBold, false) }),
			layout.Rigid(gap(8)),
			layout.Rigid(func(gtx C) D { return u.text(gtx, "Gio prototype", 13, cDim, font.Normal, false) }),
			layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
			layout.Rigid(func(gtx C) D {
				if !u.showStats {
					return D{}
				}
				s := fmt.Sprintf("%.0f fps · layout %.2f ms · startup %d ms (db %d ms) · %s",
					u.fps, float64(u.layoutCost.Microseconds())/1000, u.startup.Milliseconds(), u.loadTime.Milliseconds(), filepath.Base(u.source))
				return u.text(gtx, s, 12, cMuted, font.Normal, true)
			}),
		)
	})
}

func (u *ui) pane(gtx C, p pane, title string, count int, body layout.Widget) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			h := gtx.Dp(34)
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, h))
			paint.FillShape(gtx.Ops, cPanelBg, clip.Rect{Max: gtx.Constraints.Max}.Op())
			if u.focus == p {
				paint.FillShape(gtx.Ops, cAccent, clip.Rect{Min: image.Pt(0, h-gtx.Dp(2)), Max: image.Pt(gtx.Constraints.Max.X, h)}.Op())
			}
			return layout.Inset{Left: 14, Right: 14}.Layout(gtx, func(gtx C) D {
				col := cMuted
				if u.focus == p {
					col = cText
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return u.text(gtx, title, 13, col, font.Medium, false) }),
					layout.Rigid(gap(8)),
					layout.Rigid(func(gtx C) D { return u.text(gtx, thousands(count), 12, cDim, font.Normal, true) }),
				)
			})
		}),
		layout.Rigid(hline),
		layout.Flexed(1, body),
	)
}

// --- tracks -----------------------------------------------------------------

func (u *ui) trackPane(gtx C) D {
	return layout.Inset{Top: 4}.Layout(gtx, func(gtx C) D {
		return u.scrollList(&u.trackList).Layout(gtx, len(u.tracks), u.trackRow)
	})
}

func (u *ui) scrollList(l *widget.List) material.ListStyle {
	ls := material.List(u.th, l)
	ls.Indicator.Color = withAlpha(cText, 0x30)
	ls.Indicator.HoverColor = withAlpha(cText, 0x60)
	ls.Indicator.MinorWidth = 6
	ls.Indicator.CornerRadius = 3
	ls.Track.Color = color.NRGBA{}
	return ls
}

func (u *ui) rowState(m map[int]*rowState, i int) *rowState {
	rs, ok := m[i]
	if !ok {
		rs = &rowState{}
		m[i] = rs
	}
	rs.seen = u.frame
	return rs
}

// animate eases rs.hover toward the hover state.
func (u *ui) animate(gtx C, rs *rowState) {
	target := float32(0)
	if rs.btn.Hovered() {
		target = 1
	}
	rs.hover = u.approach(gtx, rs.hover, target, 70*time.Millisecond)
}

// approach moves v exponentially toward target and keeps frames coming until it lands.
func (u *ui) approach(gtx C, v, target float32, tau time.Duration) float32 {
	k := float32(1 - math.Exp(-float64(u.dt)/float64(tau)))
	v += (target - v) * k
	if d := target - v; d > 0.01 || d < -0.01 {
		gtx.Execute(op.InvalidateCmd{})
		return v
	}
	return target
}

func (u *ui) trackRow(gtx C, i int) D {
	rs := u.rowState(u.trackRows, i)
	for {
		c, ok := rs.btn.Update(gtx)
		if !ok {
			break
		}
		u.focus, u.selTrack = paneTracks, i
		if c.NumClicks >= 2 {
			u.play(i)
		}
	}
	u.animate(gtx, rs)
	t := u.tracks[i]
	w, h := gtx.Constraints.Max.X, gtx.Dp(30)

	return rs.btn.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(w, h))
		bg := color.NRGBA{}
		if i == u.selTrack {
			bg = cSelUnfocus
			if u.focus == paneTracks {
				bg = cSelected
			}
		}
		bg = mix(bg, cHover, rs.hover*0.7)
		pad := gtx.Dp(6)
		paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rect(pad, 1, w-pad-gtx.Dp(8), h-1), gtx.Dp(5)).Op(gtx.Ops))

		playing := i == u.playing
		titleCol, num := cText, fmt.Sprint(t.num)
		if playing {
			titleCol = cAccent
			num = "▶"
			if u.paused {
				num = "❚❚"
			}
		}
		return layout.Inset{Left: 18, Right: 24}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				fixed(gtx.Dp(26), func(gtx C) D { return u.textAligned(gtx, num, 12, cDim, font.Normal, true, text.End) }),
				layout.Rigid(gap(14)),
				layout.Flexed(0.42, func(gtx C) D { return u.text(gtx, t.title, 14, titleCol, font.Normal, false) }),
				layout.Rigid(gap(12)),
				layout.Flexed(0.26, func(gtx C) D { return u.text(gtx, t.artist, 13, cMuted, font.Normal, false) }),
				layout.Rigid(gap(12)),
				layout.Flexed(0.32, func(gtx C) D { return u.text(gtx, t.album, 13, cDim, font.Normal, false) }),
				fixed(gtx.Dp(44), func(gtx C) D { return u.textAligned(gtx, year(t.year), 12, cDim, font.Normal, true, text.End) }),
			)
		})
	})
}

// --- albums -----------------------------------------------------------------

func (u *ui) albumPane(gtx C) D {
	w := gtx.Constraints.Max.X - gtx.Dp(16)
	u.cols = max(1, w/gtx.Dp(176))
	rows := (len(u.albums) + u.cols - 1) / u.cols
	return layout.Inset{Left: 8, Top: 6}.Layout(gtx, func(gtx C) D {
		return u.scrollList(&u.albumList).Layout(gtx, rows, func(gtx C, r int) D {
			cell := (gtx.Constraints.Max.X - gtx.Dp(8)) / u.cols
			var h int
			for c := range u.cols {
				i := r*u.cols + c
				if i >= len(u.albums) {
					break
				}
				st := op.Offset(image.Pt(c*cell, 0)).Push(gtx.Ops)
				cg := gtx
				cg.Constraints = layout.Exact(image.Pt(cell, gtx.Constraints.Max.Y))
				cg.Constraints.Min.Y = 0
				h = max(h, u.albumCell(cg, i, cell).Size.Y)
				st.Pop()
			}
			return D{Size: image.Pt(gtx.Constraints.Max.X, h)}
		})
	})
}

func (u *ui) albumCell(gtx C, i, cell int) D {
	rs := u.rowState(u.albumCells, i)
	for {
		c, ok := rs.btn.Update(gtx)
		if !ok {
			break
		}
		u.focus, u.selAlbum = paneAlbums, i
		if c.NumClicks >= 2 {
			u.playAlbum(i)
		}
	}
	u.animate(gtx, rs)
	a := u.albums[i]
	pad := gtx.Dp(8)
	side := cell - 2*pad
	radius := gtx.Dp(6)

	return rs.btn.Layout(gtx, func(gtx C) D {
		return layout.UniformInset(8).Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.X = side
			gtx.Constraints.Min.X = side
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					u.drawCover(gtx, i, a, side, radius, rs.hover)
					return D{Size: image.Pt(side, side)}
				}),
				layout.Rigid(gap(8)),
				layout.Rigid(func(gtx C) D {
					col := cText
					if u.playing >= 0 && u.tracks[u.playing].albumIdx == i {
						col = cAccent
					}
					return u.text(gtx, a.title, 13, col, font.Medium, false)
				}),
				layout.Rigid(func(gtx C) D {
					return u.text(gtx, a.artist+yearSuffix(a.year), 12, cDim, font.Normal, false)
				}),
			)
		})
	})
}

func (u *ui) drawCover(gtx C, i int, a album, side, radius int, hover float32) {
	rect := image.Rect(0, 0, side, side)
	paint.FillShape(gtx.Ops, cPanelBg, clip.UniformRRect(rect, radius).Op(gtx.Ops))

	cv := u.covers.get(i, side, a.dir)
	if cv.ok {
		alpha := u.approach(gtx, u.fade[i], 1, 120*time.Millisecond)
		u.fade[i] = alpha
		cl := clip.UniformRRect(rect, radius).Push(gtx.Ops)
		layer := paint.PushOpacity(gtx.Ops, alpha)
		if sz := cv.img.Size(); sz.X != side && sz.X > 0 {
			s := float32(side) / float32(sz.X)
			tr := op2Affine(gtx, s)
			cv.img.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			tr.Pop()
		} else {
			cv.img.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
		}
		layer.Pop()
		cl.Pop()
	} else {
		delete(u.fade, i)
		if cv.done {
			lbl := u.label(initials(a.title), 28, cBorder, font.SemiBold, false)
			lbl.Alignment = text.Middle
			g := gtx
			g.Constraints = layout.Exact(image.Pt(side, side))
			layout.Center.Layout(g, lbl.Layout)
		}
	}

	// Selection ring, then a hover veil.
	if i == u.selAlbum {
		col := cBorder
		if u.focus == paneAlbums {
			col = cAccent
		}
		paint.FillShape(gtx.Ops, col, clip.Stroke{Path: clip.UniformRRect(rect, radius).Path(gtx.Ops), Width: float32(gtx.Dp(2))}.Op())
	}
	if hover > 0 {
		paint.FillShape(gtx.Ops, withAlpha(cText, uint8(hover*22)), clip.UniformRRect(rect, radius).Op(gtx.Ops))
	}
}

func op2Affine(gtx C, s float32) op.TransformStack {
	return op.Affine(f32.AffineId().Scale(f32.Pt(0, 0), f32.Pt(s, s))).Push(gtx.Ops)
}

// --- player bar -------------------------------------------------------------

func (u *ui) playerBar(gtx C) D {
	h := gtx.Dp(76)
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, h))
	paint.FillShape(gtx.Ops, cPanelBg, clip.Rect{Max: gtx.Constraints.Max}.Op())
	if u.playing < 0 {
		return layout.Center.Layout(gtx, func(gtx C) D {
			return u.text(gtx, "enter or double-click to play · tab switches pane · f toggles frame stats", 13, cDim, font.Normal, false)
		})
	}
	t := u.tracks[u.playing]
	return layout.Inset{Left: 16, Right: 20}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				side := gtx.Dp(52)
				u.drawCover(gtx, t.albumIdx, u.albums[t.albumIdx], side, gtx.Dp(5), 0)
				return D{Size: image.Pt(side, side)}
			}),
			layout.Rigid(gap(14)),
			fixed(gtx.Dp(320), func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return u.text(gtx, t.title, 14, cText, font.SemiBold, false) }),
					layout.Rigid(gap(2)),
					layout.Rigid(func(gtx C) D { return u.text(gtx, t.artist+" — "+t.album, 12, cMuted, font.Normal, false) }),
				)
			}),
			layout.Rigid(gap(28)),
			layout.Flexed(1, func(gtx C) D { return u.progress(gtx) }),
		)
	})
}

func (u *ui) progress(gtx C) D {
	total := duration(u.playing)
	frac := float32(u.elapsed) / float32(total)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		fixed(gtx.Dp(48), func(gtx C) D { return u.textAligned(gtx, clock(u.elapsed), 12, cMuted, font.Normal, true, text.End) }),
		layout.Rigid(gap(12)),
		layout.Flexed(1, func(gtx C) D {
			w, h := gtx.Constraints.Max.X, gtx.Dp(4)
			r := h / 2
			paint.FillShape(gtx.Ops, cBorder, clip.UniformRRect(image.Rect(0, 0, w, h), r).Op(gtx.Ops))
			paint.FillShape(gtx.Ops, cAccent, clip.UniformRRect(image.Rect(0, 0, int(float32(w)*frac), h), r).Op(gtx.Ops))
			knob := gtx.Dp(12)
			x := int(float32(w)*frac) - knob/2
			paint.FillShape(gtx.Ops, cText, clip.Ellipse{Min: image.Pt(x, h/2-knob/2), Max: image.Pt(x+knob, h/2+knob/2)}.Op(gtx.Ops))
			return D{Size: image.Pt(w, h)}
		}),
		layout.Rigid(gap(12)),
		fixed(gtx.Dp(48), func(gtx C) D { return u.text(gtx, clock(total), 12, cDim, font.Normal, true) }),
	)
}

// --- helpers ----------------------------------------------------------------

func (u *ui) label(s string, size unit.Sp, col color.NRGBA, weight font.Weight, monospace bool) material.LabelStyle {
	l := material.Label(u.th, size, s)
	l.Color = col
	l.Font.Weight = weight
	l.MaxLines = 1
	l.Truncator = "…"
	if monospace {
		l.Font.Typeface = mono
	}
	return l
}

// text lays out one truncated line at its natural height, so Flex can center it.
func (u *ui) text(gtx C, s string, size unit.Sp, col color.NRGBA, weight font.Weight, monospace bool) D {
	gtx.Constraints.Min.Y = 0
	return u.label(s, size, col, weight, monospace).Layout(gtx)
}

func (u *ui) textAligned(gtx C, s string, size unit.Sp, col color.NRGBA, weight font.Weight, monospace bool, a text.Alignment) D {
	l := u.label(s, size, col, weight, monospace)
	l.Alignment = a
	gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
	return l.Layout(gtx)
}

func fixed(w int, wd layout.Widget) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		gtx.Constraints.Min, gtx.Constraints.Max.X = image.Pt(w, 0), w
		return wd(gtx)
	})
}

func gap(dp unit.Dp) layout.Widget {
	return func(gtx C) D { return D{Size: image.Pt(gtx.Dp(dp), gtx.Dp(dp))} }
}

func vline(gtx C) D {
	w := max(1, gtx.Dp(1))
	paint.FillShape(gtx.Ops, cBorderSoft, clip.Rect{Max: image.Pt(w, gtx.Constraints.Max.Y)}.Op())
	return D{Size: image.Pt(w, gtx.Constraints.Max.Y)}
}

func hline(gtx C) D {
	h := max(1, gtx.Dp(1))
	paint.FillShape(gtx.Ops, cBorderSoft, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, h)}.Op())
	return D{Size: image.Pt(gtx.Constraints.Max.X, h)}
}

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func mix(a, b color.NRGBA, t float32) color.NRGBA {
	if t <= 0 {
		return a
	}
	if a.A == 0 {
		return withAlpha(b, uint8(float32(b.A)*t))
	}
	l := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t) }
	return color.NRGBA{R: l(a.R, b.R), G: l(a.G, b.G), B: l(a.B, b.B), A: l(a.A, b.A)}
}

func clock(d time.Duration) string {
	s := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func year(y int) string {
	if y == 0 {
		return ""
	}
	return fmt.Sprint(y)
}

func yearSuffix(y int) string {
	if y == 0 {
		return ""
	}
	return fmt.Sprintf(" · %d", y)
}

func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + " " + s[i:]
	}
	return s
}

func initials(s string) string {
	for _, r := range s {
		return string(r)
	}
	return "?"
}
