package display

import (
	"math"
	"math/rand"
	"testing"

	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/internal/util"
)

func TestDecorationRowsScrollAndDiff(t *testing.T) {
	for _, softwrap := range []bool{false, true} {
		t.Run(map[bool]string{false: "unwrapped", true: "wrapped"}[softwrap], func(t *testing.T) {
			w, _ := concealWindow(t, "cursor\none\nhide\nanchor\nhide\ntail", 20)
			w.Buf.Settings["softwrap"] = softwrap
			w.Buf.ConcealLines(2, 2, "hide")
			w.Buf.ConcealLines(4, 4, "hide")
			w.Buf.SetVirtualLines(3, []string{"box one", "box two"}, "box")
			w.Buf.SetVirtualLines(4, []string{"visible below hidden anchor"}, "box")
			counts := []int{1, 1, 0, 3, 1, 1}
			for line, want := range counts {
				if got := w.getRowCount(line); got != want {
					t.Fatalf("line %d rows=%d want %d", line, got, want)
				}
			}
			rows := []SLoc{{0, 0}, {1, 0}, {3, 0}, {3, 1}, {3, 2}, {4, 0}, {5, 0}}
			for i, from := range rows {
				for j, to := range rows {
					if got := w.Diff(from, to); got != j-i {
						t.Fatalf("Diff(%v,%v)=%d want %d", from, to, got, j-i)
					}
					if got := w.Scroll(from, j-i); got != to {
						t.Fatalf("Scroll(%v,%d)=%v want %v", from, j-i, got, to)
					}
				}
				if got := w.Scroll(from, math.MaxInt); got != rows[len(rows)-1] {
					t.Fatal(got)
				}
				if got := w.Scroll(from, math.MinInt); got != rows[0] {
					t.Fatal(got)
				}
			}
			if got := w.Scroll(SLoc{2, 0}, 0); got != (SLoc{3, 0}) {
				t.Fatalf("hidden start not normalized: %v", got)
			}
			if got := w.LocFromVLoc(VLoc{SLoc: SLoc{3, 1}, VisualX: 123}); got != (buffer.Loc{X: 6, Y: 3}) {
				t.Fatalf("virtual row maps to %v", got)
			}
			if got := w.LocFromVLoc(VLoc{SLoc: SLoc{4, 0}, VisualX: 7}); got != (buffer.Loc{X: 4, Y: 4}) {
				t.Fatalf("hidden anchor's virtual row maps to %v", got)
			}
			if got := w.VLocFromLoc(buffer.Loc{X: 1, Y: 2}); got.SLoc != (SLoc{3, 0}) {
				t.Fatalf("hidden text projection %v", got)
			}
		})
	}
}

func TestDecorationRowsRevealAndStaleLocations(t *testing.T) {
	w, _ := concealWindow(t, "cursor\nhidden\nanchor\nother", 20)
	w.Buf.ConcealLines(1, 1, "g")
	w.Buf.SetVirtualLines(2, []string{"one", "two"}, "g")
	w.Buf.SetVirtualLines(3, []string{"unrelated"}, "h")
	w.Buf.SetRevealMode("g", "group")
	if w.getRowCount(1) != 0 || w.getRowCount(2) != 3 {
		t.Fatal("initial decoration rows")
	}
	w.Buf.GetActiveCursor().Loc = buffer.Loc{X: 1, Y: 2}
	if w.getRowCount(1) != 1 || w.getRowCount(2) != 1 || w.getRowCount(3) != 2 {
		t.Fatal("group reveal counts")
	}
	if got := w.Scroll(SLoc{2, 2}, 0); got != (SLoc{2, 0}) {
		t.Fatalf("stale virtual row not clamped: %v", got)
	}
	w.Buf.GetActiveCursor().Loc = buffer.Loc{}
	w.Buf.GetActiveCursor().CurSelection = [2]buffer.Loc{{X: 0, Y: 1}, {X: 0, Y: 2}}
	if w.getRowCount(1) != 1 || w.getRowCount(2) != 1 {
		t.Fatal("selection group reveal")
	}
	w.Buf.GetActiveCursor().ResetSelection()
	w.Buf.ClearDecorations("g")
	if got := w.Scroll(SLoc{2, 99}, 0); got != (SLoc{2, 0}) {
		t.Fatal(got)
	}
	if w.frameReveal != nil {
		t.Fatal("mapping retained reveal snapshot")
	}
}

func TestDecorationRowsHiddenHeadTailAndEmptyView(t *testing.T) {
	w, _ := concealWindow(t, "head\nhide\nvisible\nhide\ntail", 20)
	w.Buf.GetActiveCursor().Loc = buffer.Loc{Y: 2}
	w.Buf.ConcealLines(0, 1, "h")
	w.Buf.ConcealLines(3, 4, "h")
	if got := w.Scroll(SLoc{}, 0); got != (SLoc{2, 0}) {
		t.Fatal(got)
	}
	if got := w.Scroll(SLoc{4, 0}, 0); got != (SLoc{2, 0}) {
		t.Fatal(got)
	}
	if got := w.Scroll(SLoc{2, 0}, 100); got != (SLoc{2, 0}) {
		t.Fatal(got)
	}
	if got := w.Scroll(SLoc{2, 0}, -100); got != (SLoc{2, 0}) {
		t.Fatal(got)
	}
	w.Buf.ConcealLines(2, 2, "h")
	cursors := w.Buf.GetCursors()
	w.Buf.SetCursors(nil)
	defer w.Buf.SetCursors(cursors)
	for _, n := range []int{0, 1, -1, math.MaxInt, math.MinInt} {
		if got := w.Scroll(SLoc{3, 4}, n); got != (SLoc{}) {
			t.Fatalf("all hidden scroll %d: %v", n, got)
		}
	}
	if got := w.Diff(SLoc{}, SLoc{4, 0}); got != 0 {
		t.Fatal(got)
	}
	if got := w.VLocFromLoc(buffer.Loc{Y: 4}); got.SLoc != (SLoc{}) {
		t.Fatal(got)
	}
}

func TestDecorationRowsVirtualHitTestingUsesRowModel(t *testing.T) {
	w, _ := concealWindow(t, "cursor\nHIDDEN tail", 20)
	w.Buf.AddConceal(1, 0, 6, "X", "", "c")
	w.Buf.SetVirtualLines(1, []string{"box"}, "v")
	if got := w.LocFromVisual(buffer.Loc{X: 0, Y: 2}); got != (buffer.Loc{X: 11, Y: 1}) {
		t.Fatalf("virtual click bypassed row model: %v", got)
	}
	if got := w.LocFromVLoc(VLoc{SLoc: SLoc{1, 0}, VisualX: 2}); got != (buffer.Loc{X: 7, Y: 1}) {
		t.Fatal(got)
	}
	if got := w.VLocFromLoc(buffer.Loc{X: 7, Y: 1}); got != (VLoc{SLoc: SLoc{1, 0}, VisualX: 2}) {
		t.Fatal(got)
	}
	w.Buf.Settings["conceal"] = false
	if w.getRowCount(1) != 1 {
		t.Fatal("disabled decorations changed rows")
	}
	if got := w.Scroll(SLoc{}, math.MaxInt); got != (SLoc{1, 0}) {
		t.Fatal(got)
	}
	if got := w.Scroll(SLoc{1, 0}, math.MinInt); got != (SLoc{}) {
		t.Fatal(got)
	}
	if got := w.LocFromVLoc(VLoc{SLoc: SLoc{1, 0}, VisualX: 2}); got != (buffer.Loc{X: 2, Y: 1}) {
		t.Fatal(got)
	}
}

func TestConcealedRowsWrapAndMapSource(t *testing.T) {
	w, sim := concealWindow(t, "c\nabHIDDEN xy\nTAIL", 6)
	w.Buf.Settings["softwrap"] = true
	w.Buf.AddConceal(1, 2, 8, "12345678", "", "g")
	redraw(w)
	if got := w.getRowCount(1); got != 3 {
		t.Fatalf("concealed rows %d", got)
	}
	if got := w.SLocFromLoc(buffer.Loc{Y: 2}); w.Diff(SLoc{}, got) != 4 {
		t.Fatalf("tail scrolling distance: %v %d", got, w.Diff(SLoc{}, got))
	}
	if got := screenRow(sim, 4, 6); got != "TAIL" {
		t.Fatalf("tail draw: %q", got)
	}
	// Every cell of a wrapped replacement maps to its source start.
	for _, vloc := range []VLoc{{SLoc{1, 0}, 2}, {SLoc{1, 0}, 5}, {SLoc{1, 1}, 0}, {SLoc{1, 1}, 3}} {
		if got := w.LocFromVLoc(vloc); got != (buffer.Loc{X: 2, Y: 1}) {
			t.Fatalf("%v maps to %v", vloc, got)
		}
	}
	for _, col := range []int{2, 3, 7} {
		if got := w.VLocFromLoc(buffer.Loc{X: col, Y: 1}); got != (VLoc{SLoc{1, 0}, 2}) {
			t.Fatalf("concealed source %d maps to %v", col, got)
		}
	}
	if got := w.VLocFromLoc(buffer.Loc{X: 8, Y: 1}); got != (VLoc{SLoc{1, 1}, 4}) {
		t.Fatal(got)
	}
	if got := w.VLocFromLoc(buffer.Loc{X: 11, Y: 1}); got != (VLoc{SLoc{1, 2}, 1}) {
		t.Fatal(got)
	}
	// Cursor reveal restores source widths and suppresses its virtual rows.
	w.Buf.GetActiveCursor().Loc = buffer.Loc{Y: 1}
	if got := w.getRowCount(1); got != 2 {
		t.Fatalf("revealed row count %d", got)
	}
}

func TestConcealedRowsEmptyAndWordPadding(t *testing.T) {
	w, _ := concealWindow(t, "c\nHIDDEN\naa HIDDENbbbb", 6)
	w.Buf.Settings["softwrap"] = true
	w.Buf.Settings["wordwrap"] = true
	w.Buf.AddConceal(1, 0, 6, "", "", "g")
	w.Buf.AddConceal(2, 3, 9, "b", "", "g")
	if w.getTextRowCount(1) != 1 {
		t.Fatal("empty concealed line still needs one text row")
	}
	if got := w.LocFromVLoc(VLoc{SLoc{1, 0}, 0}); got != (buffer.Loc{X: 6, Y: 1}) {
		t.Fatal(got)
	}
	if got := w.LocFromVLoc(VLoc{SLoc{2, 0}, 5}); got != (buffer.Loc{X: 2, Y: 2}) {
		t.Fatalf("word padding mapped to next row: %v", got)
	}
	if got := w.LocFromVLoc(VLoc{SLoc{2, 1}, 0}); got != (buffer.Loc{X: 3, Y: 2}) {
		t.Fatal(got)
	}
}

func TestConcealedRowsAgreeWithDrawing(t *testing.T) {
	rng := rand.New(rand.NewSource(73))
	replacements := []string{"", "✔", "XYZ", "long replacement", "界", "e\u0301", "\t", "x \t y"}
	for i := 0; i < 100; i++ {
		width := 2 + rng.Intn(18)
		replacement := replacements[rng.Intn(len(replacements))]
		w, sim := concealWindow(t, "c\nabHIDDEN\tx界 tail\nZ", width)
		w.Buf.Settings["softwrap"] = true
		w.Buf.Settings["wordwrap"] = i%2 == 0
		w.Buf.AddConceal(1, 2, 8, replacement, "", "g")
		redraw(w)
		rowCount := w.getTextRowCount(1)
		if 1+rowCount < 10 {
			if got := screenRow(sim, 1+rowCount, width); got != "Z" {
				t.Fatalf("case %d width %d word=%v replacement %q rowCount %d: tail at wrong row %q", i, width, w.Buf.Settings["wordwrap"], replacement, rowCount, got)
			}
		}
		data := w.Buf.LineBytes(1)
		for col := 8; col < util.CharacterCount(data); col++ {
			ch, _, _ := util.DecodeCharacter(util.SliceEnd(data, col))
			if ch == '\t' || ch == ' ' {
				continue
			}
			loc := w.VLocFromLoc(buffer.Loc{X: col, Y: 1})
			if loc.Row+1 >= 10 {
				continue
			}
			got, _, _, _ := sim.GetContent(loc.VisualX, loc.Row+1)
			if got != ch {
				t.Fatalf("case %d width %d word=%v replacement %q source %d: mapping %v draws %q want %q", i, width, w.Buf.Settings["wordwrap"], replacement, col, loc, got, ch)
			}
			if back := w.LocFromVLoc(loc); back != (buffer.Loc{X: col, Y: 1}) {
				t.Fatalf("round trip %v -> %v", loc, back)
			}
		}
	}
}

func TestDecorationRowsZeroWidthAndSnapshotOwnership(t *testing.T) {
	w, _ := concealWindow(t, "c\nHIDDEN\nlast", 20)
	w.Buf.Settings["softwrap"] = true
	w.Buf.AddConceal(1, 0, 6, "very long replacement", "", "g")
	w.Buf.SetVirtualLines(1, []string{"one", "two"}, "v")
	w.bufWidth = 0
	if got := w.getRowCount(1); got != 3 {
		t.Fatal(got)
	}
	if got := w.LocFromVLoc(VLoc{SLoc{1, 0}, 4}); got != (buffer.Loc{Y: 1}) {
		t.Fatal(got)
	}
	if got := w.LocFromVLoc(VLoc{SLoc{1, 2}, 4}); got != (buffer.Loc{X: 6, Y: 1}) {
		t.Fatal(got)
	}
	snapshot := w.computeDecorationReveal()
	w.frameReveal = snapshot
	w.Scroll(SLoc{}, 2)
	w.VLocFromLoc(buffer.Loc{Y: 1})
	if w.frameReveal != snapshot {
		t.Fatal("mapping cleared drawing's reveal snapshot")
	}
	w.frameReveal = nil
	w.Buf.Settings["softwrap"] = false
	w.Buf.Settings["conceal"] = false
	if got := w.Scroll(SLoc{1, 2}, 0); got != (SLoc{1, 0}) {
		t.Fatal(got)
	}
}

func TestDecorationRowsIndependentSplits(t *testing.T) {
	w, _ := concealWindow(t, "c\nhidden\nanchor", 20)
	w.Buf.ConcealLines(1, 1, "g")
	w.Buf.SetVirtualLines(2, []string{"box"}, "g")
	w.Buf.SetRevealMode("g", "group")
	other := buffer.NewBufferFromString("", "", buffer.BTDefault)
	t.Cleanup(other.Close)
	other.SharedBuffer = w.Buf.SharedBuffer
	other.Settings = w.Buf.Settings
	other.GetActiveCursor().Loc = buffer.Loc{Y: 2}
	second := NewBufWindow(0, 0, 20, 12, other)
	second.bufWidth = 20
	if w.getRowCount(1) != 0 || w.getRowCount(2) != 2 {
		t.Fatal("first window decoration layout")
	}
	if second.getRowCount(1) != 1 || second.getRowCount(2) != 1 {
		t.Fatal("second window reveal layout")
	}
}
