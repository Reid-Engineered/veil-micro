package display

import (
	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/internal/config"
	"github.com/micro-editor/tcell/v2"
	"math/rand"
	"strings"
	"testing"
)

func TestVirtualDrawingRowsAndClicks(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "wrapped"}[wrap], func(t *testing.T) {
			w, sim := concealWindow(t, "cursor\nabcdefgh\nhidden\nlast", 8)
			w.Buf.Settings["softwrap"] = wrap
			w.Buf.SetVirtualLines(1, []string{"box1", "box2"}, "a")
			w.Buf.SetVirtualLines(1, []string{"box3"}, "b")
			w.Buf.ConcealLines(2, 2, "hide")
			w.Buf.SetVirtualLines(2, []string{"hidden anchor"}, "a")
			redraw(w)
			want := []string{"cursor", "abcdefgh", "box1", "box2", "box3", "hidden a", "last"}
			if wrap {
				want = []string{"cursor", "abcdefgh", "", "box1", "box2", "box3", "hidden a", "last"}
			}
			for y, s := range want {
				if got := screenRow(sim, y, 8); got != s {
					t.Fatalf("row %d: %q != %q", y, got, s)
				}
			}
			boxY := 2
			if wrap {
				boxY++
			}
			if got := w.LocFromVisual(buffer.Loc{X: 3, Y: boxY}); got != (buffer.Loc{X: 8, Y: 1}) {
				t.Fatal(got)
			}
			hiddenY := 5
			if wrap {
				hiddenY++
			}
			if got := w.LocFromVisual(buffer.Loc{Y: hiddenY}); got != (buffer.Loc{X: 6, Y: 2}) {
				t.Fatal(got)
			}
			w.StartLine = SLoc{1, w.getTextRowCount(1) + 1}
			redraw(w)
			if got := screenRow(sim, 0, 8); got != "box2" {
				t.Fatal(got)
			}
			if got := w.LocFromVisual(buffer.Loc{}); got != (buffer.Loc{X: 8, Y: 1}) {
				t.Fatal(got)
			}
		})
	}
}

func TestVirtualDrawingStylesUnicodeAndClip(t *testing.T) {
	w, sim := concealWindow(t, "cursor\nanchor\nlast", 9)
	w.Buf.Settings["ruler"] = true
	w.Buf.Settings["diffgutter"] = true
	w.updateDisplayInfo()
	w.Buf.SetVirtualLines(1, []string{"a\t界éZTAIL"}, "v")
	red := tcell.StyleDefault.Foreground(tcell.ColorRed)
	green := tcell.StyleDefault.Foreground(tcell.ColorGreen)
	config.Colorscheme["red"], config.Colorscheme["green"] = red, green
	w.Buf.StyleVirtualLine(1, 0, 0, -1, "red", "v")
	w.Buf.StyleVirtualLine(1, 0, 2, 4, "green", "v")
	w.Buf.StyleVirtualLine(1, 0, 4, 5, "missing", "v")
	w.StartCol = 3
	redraw(w)
	if got := screenRow(sim, 2, 9); got != "    界 éZT" {
		t.Fatalf("%q", got)
	}
	for x := 0; x < w.gutterOffset; x++ {
		_, _, style, _ := sim.GetContent(x, 2)
		if style != config.DefStyle {
			t.Fatal("gutter styled")
		}
	}
	for x, want := range map[int]tcell.Style{3: red, 4: green, 5: green, 6: green, 7: config.DefStyle} {
		_, _, got, _ := sim.GetContent(x, 2)
		if got != want {
			t.Fatalf("cell %d style %v != %v", x, got, want)
		}
	}
	// Virtual rows truncate rather than wrapping, even when text softwrap is on.
	w.Buf.Settings["softwrap"] = true
	w.StartCol = 0
	redraw(w)
	if got := screenRow(sim, 4, 9); strings.Contains(got, "TAIL") {
		t.Fatal("virtual row wrapped")
	}
}

func TestVirtualRevealAndStaleStart(t *testing.T) {
	w, sim := concealWindow(t, "cursor\nraw1\nraw2\nlast", 12)
	w.Buf.ConcealLines(1, 2, "board")
	w.Buf.SetVirtualLines(1, []string{"board1", "board2"}, "board")
	w.Buf.SetRevealMode("board", "group")
	w.StartLine = SLoc{1, 1}
	redraw(w)
	if got := screenRow(sim, 0, 12); got != "board2" {
		t.Fatal(got)
	}
	w.Buf.GetActiveCursor().Loc = buffer.Loc{Y: 2}
	redraw(w)
	if w.StartLine != (SLoc{1, 0}) || screenRow(sim, 0, 12) != "raw1" || screenRow(sim, 1, 12) != "raw2" {
		t.Fatal("group reveal/stale start", w.StartLine)
	}
	w.Buf.GetActiveCursor().Loc = buffer.Loc{}
	w.Buf.GetActiveCursor().CurSelection = [2]buffer.Loc{{Y: 1}, {X: 1, Y: 2}}
	redraw(w)
	if screenRow(sim, 0, 12) != "raw1" {
		t.Fatal("selection reveal")
	}
	w.Buf.GetActiveCursor().CurSelection = [2]buffer.Loc{}
	w.Buf.Settings["conceal"] = false
	redraw(w)
	if screenRow(sim, 0, 12) != "raw1" {
		t.Fatal("disabled decorations")
	}
}

func TestVirtualDrawingConfinedToWindow(t *testing.T) {
	w, sim := concealWindow(t, "cursor\nabcdefghijk\nlast", 4)
	w.Y = 3
	w.Height = 4
	w.updateDisplayInfo()
	w.Buf.Settings["softwrap"] = true
	w.Buf.SetVirtualLines(1, []string{"v1", "v2", "v3", "v4"}, "v")
	w.StartLine = SLoc{1, 1}
	for y := 0; y < 20; y++ {
		for x := 0; x < 4; x++ {
			sim.SetContent(x, y, '!', nil, config.DefStyle)
		}
	}
	redraw(w)
	for _, y := range []int{0, 1, 2, 7, 8} {
		if got := screenRow(sim, y, 4); got != "!!!!" {
			t.Fatalf("outside row %d: %q", y, got)
		}
	}
	if got := screenRow(sim, 3, 4); got != "efgh" {
		t.Fatal(got)
	}
	// Start on a virtual row without processing offscreen source rows as visible.
	w.StartLine = SLoc{1, 4}
	redraw(w)
	for _, y := range []int{0, 1, 2, 7, 8} {
		if got := screenRow(sim, y, 4); got != "!!!!" {
			t.Fatalf("outside row %d after virtual start: %q", y, got)
		}
	}
	if got := screenRow(sim, 3, 4); got != "v2" {
		t.Fatal(got)
	}
}

func TestCursorScrollSkipsVirtualAndHiddenRows(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		w, _ := concealWindow(t, "cursor\nanchor\nhidden\nnext\ntail", 8)
		w.Buf.Settings["softwrap"] = wrap
		w.Buf.SetVirtualLines(1, []string{"v1", "v2"}, "v")
		w.Buf.ConcealLines(2, 2, "h")
		w.Buf.ConcealLines(4, 4, "h")
		w.Buf.SetVirtualLines(4, []string{"tail1", "tail2"}, "v")
		if got := w.CursorScroll(SLoc{1, 0}, 1); got != (SLoc{3, 0}) {
			t.Fatal(got)
		}
		if got := w.CursorScroll(SLoc{3, 0}, -1); got != (SLoc{1, 0}) {
			t.Fatal(got)
		}
		if got := w.CursorScroll(SLoc{3, 0}, 1); got != (SLoc{3, 0}) {
			t.Fatal(got)
		}
		if got := w.CursorScroll(SLoc{}, 3); got != (SLoc{3, 0}) {
			t.Fatal(got)
		}
	}
}

func TestVirtualDrawingMatchesFlattenedRows(t *testing.T) {
	rng := rand.New(rand.NewSource(20261012))
	for sample := 0; sample < 40; sample++ {
		width := 3 + rng.Intn(8)
		w, sim := concealWindow(t, "c\nabcHIDDEN xy\nhidden\nend", width)
		wrap := sample%2 == 0
		w.Buf.Settings["softwrap"] = wrap
		w.Buf.Settings["wordwrap"] = sample%3 == 0
		replacement := []string{"", "!", "UV"}[rng.Intn(3)]
		w.Buf.AddConceal(1, 3, 9, replacement, "", "g")
		w.Buf.SetVirtualLines(1, []string{"virtualABCDEFGHI", "V2"}, "v")
		w.Buf.ConcealLines(2, 2, "h")
		w.Buf.SetVirtualLines(2, []string{"hiddenAnchor"}, "v")
		// Independent reference: ordinary drawing of substituted text, with rows
		// flattened explicitly in text/virtual order (virtual strings are clipped).
		controlBuf := buffer.NewBufferFromString("c\nabc"+replacement+" xy\nend", "", buffer.BTDefault)
		controlBuf.Settings = w.Buf.Settings
		control := NewBufWindow(0, 0, width, 12, controlBuf)
		control.updateDisplayInfo()
		redraw(control)
		first := control.getTextRowCount(0)
		count := control.getTextRowCount(1)
		expected := []string{}
		for y := 0; y < first+count; y++ {
			expected = append(expected, screenRow(sim, y, width))
		}
		for _, s := range []string{"virtualABCDEFGHI", "V2", "hiddenAnchor"} {
			if len(s) > width {
				s = s[:width]
			}
			expected = append(expected, s)
		}
		expected = append(expected, "end")
		// Width 3 leaves micro's existing blank EOL row when wrapping is on.
		if wrap && width == 3 {
			expected = append(expected, "")
		}
		controlBuf.Close()
		start := w.Scroll(SLoc{}, 0)
		for row, want := range expected {
			w.StartLine = start
			redraw(w)
			if got := screenRow(sim, 0, width); got != want {
				t.Fatalf("sample %d row %d at %v: %q != %q", sample, row, start, got, want)
			}
			if row+1 < len(expected) {
				start = w.Scroll(start, 1)
			}
		}
	}
}

func TestVirtualNarrowResizeAndRelocate(t *testing.T) {
	w, sim := concealWindow(t, "cursor\nanchor\nlast", 8)
	w.Buf.SetVirtualLines(1, []string{"界", "tail"}, "v")
	w.StartLine = SLoc{1, 2}
	w.Buf.SetVirtualLines(1, []string{"界"}, "v")
	w.Buf.Settings["scrollmargin"] = float64(0)
	w.Resize(1, 5)
	redraw(w)
	// A wide rune cannot draw into an adjacent split when only one cell fits.
	w.StartLine = SLoc{1, 1}
	sim.SetContent(1, 0, '!', nil, config.DefStyle)
	redraw(w)
	ch, _, _, _ := sim.GetContent(1, 0)
	if ch != '!' {
		t.Fatal("wide glyph escaped window")
	}
	w.Resize(0, 0)
	w.Display()
	w.Resize(8, 5)
	w.Buf.GetActiveCursor().Loc = buffer.Loc{Y: 2}
	w.Relocate()
	redraw(w)
	row := w.Diff(w.StartLine, w.SLocFromLoc(w.Buf.GetActiveCursor().Loc))
	if row < 0 || row >= w.bufHeight {
		t.Fatal("cursor outside relocated view", row)
	}
}
