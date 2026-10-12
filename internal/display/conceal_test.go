package display

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/internal/config"
	ulua "github.com/micro-editor/micro/v2/internal/lua"
	"github.com/micro-editor/micro/v2/internal/screen"
	"github.com/micro-editor/micro/v2/internal/util"
	"github.com/micro-editor/micro/v2/pkg/highlight"
	"github.com/micro-editor/tcell/v2"
	lua "github.com/yuin/gopher-lua"
)

func concealWindow(t *testing.T, text string, width int) (*BufWindow, tcell.SimulationScreen) {
	t.Helper()
	oldSettings := config.GlobalSettings
	config.InitRuntimeFiles(false)
	if err := config.InitGlobalSettings(); err != nil {
		t.Fatal(err)
	}
	config.GlobalSettings["syntax"] = false
	config.GlobalSettings["savecursor"] = false
	config.GlobalSettings["saveundo"] = false
	config.GlobalSettings["backup"] = false
	config.GlobalSettings["fastdirty"] = true
	config.GlobalSettings["fakecursor"] = false
	oldConfigDir := config.ConfigDir
	config.ConfigDir = t.TempDir()
	oldLua, oldScreen, oldColors, oldStyle := ulua.L, screen.Screen, config.Colorscheme, config.DefStyle
	ulua.L = lua.NewState()
	config.Colorscheme = make(map[string]tcell.Style)
	config.DefStyle = tcell.StyleDefault
	sim, err := screen.InitSimScreen()
	if err != nil {
		t.Fatal(err)
	}
	sim.SetSize(width+8, 20)
	b := buffer.NewBufferFromString(text, "", buffer.BTDefault)
	b.Settings["conceal"] = true
	b.Settings["ruler"] = false
	b.Settings["scrollbar"] = false
	b.Settings["statusline"] = true
	b.Settings["matchbrace"] = false
	b.Settings["cursorline"] = false
	b.Settings["softwrap"] = false
	b.Settings["wordwrap"] = false
	w := NewBufWindow(0, 0, width, 12, b)
	w.updateDisplayInfo()
	t.Cleanup(func() {
		b.Close()
		config.GlobalSettings = oldSettings
		config.ConfigDir = oldConfigDir
		ulua.L.Close()
		sim.Fini()
		ulua.L, screen.Screen, config.Colorscheme, config.DefStyle = oldLua, oldScreen, oldColors, oldStyle
	})
	return w, sim
}

func screenRow(sim tcell.SimulationScreen, y, width int) string {
	var s strings.Builder
	for x := 0; x < width; x++ {
		ch, combining, _, _ := sim.GetContent(x, y)
		if ch == 0 {
			ch = ' '
		}
		s.WriteRune(ch)
		for _, c := range combining {
			s.WriteRune(c)
		}
	}
	return strings.TrimRight(s.String(), " ")
}

func redraw(w *BufWindow) { w.Clear(); w.displayBuffer() }

func TestConcealDrawingAndDisabledPath(t *testing.T) {
	w, sim := concealWindow(t, "cursor\n- [x] task\nplain", 20)
	if !w.Buf.AddConceal(1, 0, 5, "✔", "special", "checkbox") {
		t.Fatal("conceal rejected")
	}
	config.Colorscheme["special"] = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	w.Buf.HighlightSearch = true
	redraw(w)
	if got := screenRow(sim, 1, 20); got != "✔ task" {
		t.Fatalf("got %q", got)
	}
	_, _, style, _ := sim.GetContent(0, 1)
	if style != config.Colorscheme["special"] {
		t.Fatal("replacement style lost")
	}
	if loc := w.LocFromVisual(buffer.Loc{X: 2, Y: 1}); loc != (buffer.Loc{X: 6, Y: 1}) {
		t.Fatalf("trailing text maps to %v", loc)
	}
	if loc := w.LocFromVisual(buffer.Loc{X: 0, Y: 1}); loc != (buffer.Loc{X: 0, Y: 1}) {
		t.Fatalf("replacement maps to %v", loc)
	}
	w.Buf.Settings["conceal"] = false
	redraw(w)
	if got := screenRow(sim, 1, 20); got != "- [x] task" {
		t.Fatalf("disabled: %q", got)
	}
	if w.concealGlyphs(1) != nil {
		t.Fatal("disabled path allocated display glyphs")
	}
	if loc := w.LocFromVisual(buffer.Loc{X: 6, Y: 1}); loc != (buffer.Loc{X: 6, Y: 1}) {
		t.Fatal(loc)
	}
}

func TestConcealRevealCursorSelectionAndGroups(t *testing.T) {
	w, sim := concealWindow(t, "cursor\nfirst\nsecond\nanchor\nend", 20)
	w.Buf.AddConceal(1, 0, 5, "ONE", "", "g")
	w.Buf.AddConceal(2, 0, 6, "TWO", "", "g")
	redraw(w)
	if screenRow(sim, 1, 20) != "ONE" || screenRow(sim, 2, 20) != "TWO" {
		t.Fatal("not concealed")
	}
	w.Buf.GetActiveCursor().Loc = buffer.Loc{X: 2, Y: 1}
	redraw(w)
	if screenRow(sim, 1, 20) != "first" || screenRow(sim, 2, 20) != "TWO" {
		t.Fatal("line reveal leaked")
	}
	x, y, visible := sim.GetCursor()
	if !visible || x != 2 || y != 1 {
		t.Fatalf("cursor = %d,%d,%v", x, y, visible)
	}
	w.Buf.SetRevealMode("g", "group")
	redraw(w)
	if screenRow(sim, 2, 20) != "second" {
		t.Fatal("group did not reveal")
	}
	w.Buf.GetActiveCursor().Loc = buffer.Loc{}
	w.Buf.SetVirtualLines(3, []string{"virtual"}, "g")
	w.Buf.GetActiveCursor().CurSelection = [2]buffer.Loc{{X: 1, Y: 3}, {X: 0, Y: 4}}
	redraw(w)
	if screenRow(sim, 1, 20) != "first" {
		t.Fatal("virtual anchor selection did not reveal group")
	}
	// A reversed selection must have the same effect, including offscreen anchors.
	c := w.Buf.GetActiveCursor()
	c.CurSelection[0], c.CurSelection[1] = c.CurSelection[1], c.CurSelection[0]
	w.StartLine.Line = 1
	redraw(w)
	if screenRow(sim, 0, 20) != "first" {
		t.Fatal("reversed/offscreen group reveal failed")
	}
	c.ResetSelection()
	w.Buf.SetRevealMode("g", "line")
	c.CurSelection = [2]buffer.Loc{{X: 0, Y: 0}, {X: 0, Y: 1}}
	if w.concealGlyphs(1) == nil {
		t.Fatal("exclusive end of selection incorrectly touched next line")
	}
	w.Buf.AddCursor(buffer.NewCursor(w.Buf, buffer.Loc{X: 1, Y: 2}))
	if w.concealGlyphs(2) != nil {
		t.Fatal("secondary cursor did not reveal its line")
	}
}

func TestConcealRevealIndependentSplits(t *testing.T) {
	w, _ := concealWindow(t, "cursor\nconceal", 20)
	w.Buf.AddConceal(1, 0, 7, "X", "", "g")
	other := buffer.NewBufferFromString("", "", buffer.BTDefault)
	t.Cleanup(other.Close)
	other.SharedBuffer = w.Buf.SharedBuffer
	other.Settings = w.Buf.Settings
	other.GetActiveCursor().Loc = buffer.Loc{X: 1, Y: 1}
	second := NewBufWindow(0, 0, 20, 12, other)
	if w.concealGlyphs(1) == nil || second.concealGlyphs(1) != nil {
		t.Fatal("reveal state shared between windows")
	}
	second.SetActive(false)
	if second.concealGlyphs(1) != nil {
		t.Fatal("inactive window cursor should still reveal")
	}
}

func TestConcealHorizontalClipping(t *testing.T) {
	cases := []struct {
		text, replacement  string
		start, end, scroll int
		want               string
		source             int
	}{
		{"abHIDDENcd", "XYZ", 2, 8, 3, "YZcd", 2},
		{"abHIDDENcd", "XYZ", 2, 8, 5, "cd", 8},
		{"abHIDDENcd", "", 2, 8, 2, "cd", 8},
		{"abHIDDENcd", "", 2, 8, 20, "", 10},
		{"HIDDEN\tZ", "x", 0, 6, 2, "  Z", 6},
		{"HIDDENz", "界", 0, 6, 1, " z", 0},
		{"HIDDENz", "e\u0301", 0, 6, 0, "e\u0301z", 0},
	}
	for _, tc := range cases {
		t.Run(tc.replacement+tc.want, func(t *testing.T) {
			w, sim := concealWindow(t, "cursor\n"+tc.text, 20)
			w.Buf.AddConceal(1, tc.start, tc.end, tc.replacement, "missing-style", "g")
			w.StartCol = tc.scroll
			redraw(w)
			if got := screenRow(sim, 1, 20); got != tc.want {
				t.Fatalf("scroll %d: got %q want %q", tc.scroll, got, tc.want)
			}
			if got := w.LocFromVisual(buffer.Loc{X: 0, Y: 1}); got.X != tc.source {
				t.Fatalf("mapping %v, want col %d", got, tc.source)
			}
			_, _, style, _ := sim.GetContent(0, 1)
			if style != config.DefStyle {
				t.Fatal("unknown style did not fall back")
			}
		})
	}
}

func TestConcealEmptyAdjacentAndUnicodeRanges(t *testing.T) {
	w, sim := concealWindow(t, "cursor\néa\u0301hidden界end\nremove", 20)
	w.Buf.AddConceal(1, 1, 2, "Å", "", "a")
	w.Buf.AddConceal(1, 2, 8, "", "", "b")
	w.Buf.AddConceal(1, 8, 9, "WIDE", "", "c")
	w.Buf.AddConceal(2, 0, 6, "", "", "a")
	redraw(w)
	if got := screenRow(sim, 1, 20); got != "éÅWIDEend" {
		t.Fatal(got)
	}
	if got := screenRow(sim, 2, 20); got != "" {
		t.Fatal(got)
	}
	if loc := w.LocFromVisual(buffer.Loc{X: 4, Y: 1}); loc.X != 8 {
		t.Fatal(loc)
	}
	if loc := w.LocFromVisual(buffer.Loc{X: 0, Y: 2}); loc.X != 6 {
		t.Fatal(loc)
	}
}

func TestConcealSoftwrapDrawing(t *testing.T) {
	for _, wordwrap := range []bool{false, true} {
		t.Run(map[bool]string{false: "character", true: "word"}[wordwrap], func(t *testing.T) {
			w, sim := concealWindow(t, "cur\nabcHIDDEN xy", 6)
			w.Buf.Settings["softwrap"] = true
			w.Buf.Settings["wordwrap"] = wordwrap
			w.Buf.AddConceal(1, 3, 9, "UV", "", "g")
			redraw(w)
			if got := screenRow(sim, 1, 6); got != "abcUV" {
				t.Fatal(got)
			}
			if got := screenRow(sim, 2, 6); got != "xy" {
				t.Fatal(got)
			}
		})
	}
}

func TestConcealPreservesPlainAndReplacementStyles(t *testing.T) {
	w, sim := concealWindow(t, "cursor\nHIDDEN tail", 20)
	green := tcell.StyleDefault.Foreground(tcell.ColorGreen)
	config.Colorscheme["special"] = green
	w.Buf.AddConceal(1, 0, 6, "X", "special", "g")
	w.Buf.Settings["showchars"] = "space=·"
	redraw(w)
	_, _, style, _ := sim.GetContent(0, 1)
	if style != green {
		t.Fatal("replacement style")
	}
	if got := screenRow(sim, 1, 20); got != "X·tail" {
		t.Fatal(got)
	}
	w.Buf.ClearDecorations("")
	redraw(w)
	if got := screenRow(sim, 1, 20); got != "HIDDEN·tail" {
		t.Fatal(got)
	}
}

func TestConcealGlyphWidthsAgreeWithDisplayedCells(t *testing.T) {
	// Independently substitute source text, then compare every horizontal start
	// against micro's existing character/cell conversions, including tabs/Unicode.
	rng := rand.New(rand.NewSource(42))
	replacements := []string{"", "✔", "XYZ", "界", "e\u0301", "\t"}
	for i := 0; i < 40; i++ {
		replacement := replacements[rng.Intn(len(replacements))]
		w, _ := concealWindow(t, "cursor\néHIDDEN\t界 tail", 30)
		w.Buf.AddConceal(1, 1, 7, replacement, "", "g")
		expected := []byte("é" + replacement + "\t界 tail")
		glyphs := w.concealGlyphs(1)
		var rendered strings.Builder
		for _, g := range glyphs {
			rendered.WriteRune(g.r)
			for _, c := range g.combc {
				rendered.WriteRune(c)
			}
		}
		if rendered.String() != string(expected) {
			t.Fatal(rendered.String())
		}
		for x := 0; x < 20; x++ {
			idx := displayGlyphStart(glyphs, x)
			expectedCol := util.GetCharPosInLine(expected, x, 4)
			if idx != expectedCol {
				t.Fatalf("%q x=%d: glyph index %d want %d", expected, x, idx, expectedCol)
			}
		}
	}
}

func TestConcealDrawingMatchesSubstitutedText(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	replacements := []string{"", "✔", "XYZ", "界", "e\u0301", "\t", " a b "}
	for i := 0; i < 80; i++ {
		replacement := replacements[rng.Intn(len(replacements))]
		width := 2 + rng.Intn(18)
		w, sim := concealWindow(t, "c\néHIDDEN\t界 end", width)
		w.Buf.Settings["softwrap"] = i%2 == 0
		w.Buf.Settings["wordwrap"] = i%3 == 0
		if !w.Buf.Settings["softwrap"].(bool) {
			w.StartCol = rng.Intn(12)
		}
		w.Buf.AddConceal(1, 1, 7, replacement, "", "g")
		sim.Clear()
		redraw(w)
		type cell struct {
			r         rune
			combining string
			style     tcell.Style
		}
		got := make([]cell, 0)
		for y := 0; y < 10; y++ {
			for x := 0; x < width; x++ {
				ch, comb, style, _ := sim.GetContent(x, y)
				got = append(got, cell{ch, string(comb), style})
			}
		}
		controlBuf := buffer.NewBufferFromString("c\né"+replacement+"\t界 end", "", buffer.BTDefault)
		controlBuf.Settings = w.Buf.Settings
		control := NewBufWindow(0, 0, width, 12, controlBuf)
		control.StartCol = w.StartCol
		control.updateDisplayInfo()
		sim.Clear()
		redraw(control)
		idx := 0
		for y := 0; y < 10; y++ {
			for x := 0; x < width; x++ {
				ch, comb, style, _ := sim.GetContent(x, y)
				want := cell{ch, string(comb), style}
				if got[idx] != want {
					t.Fatalf("sample %d width %d replacement %q scroll %d wrap=%v word=%v at %d,%d: got %#v want %#v", i, width, replacement, w.StartCol, w.Buf.Settings["softwrap"], w.Buf.Settings["wordwrap"], x, y, got[idx], want)
				}
				idx++
			}
		}
		controlBuf.Close()
	}
}

func TestConcealStyleTransitionInHiddenSource(t *testing.T) {
	w, sim := concealWindow(t, "c\nAAhiddenZZ", 20)
	oldGroups := highlight.Groups
	highlight.Groups = map[string]highlight.Group{"before": 1, "after": 2}
	t.Cleanup(func() { highlight.Groups = oldGroups })
	config.Colorscheme["before"] = tcell.StyleDefault.Foreground(tcell.ColorRed)
	config.Colorscheme["after"] = tcell.StyleDefault.Foreground(tcell.ColorBlue)
	config.Colorscheme["replacement"] = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	w.Buf.SetMatch(1, highlight.LineMatch{0: 1, 4: 2})
	w.Buf.AddConceal(1, 2, 8, "!", "replacement", "g")
	redraw(w)
	for x, name := range []string{"before", "before", "replacement", "after", "after"} {
		_, _, style, _ := sim.GetContent(x, 1)
		if style != config.Colorscheme[name] {
			t.Fatalf("cell %d lost %s style: got %#v want %#v matches=%v groups=%v", x, name, style, config.Colorscheme[name], w.Buf.Match(1), highlight.Groups)
		}
	}
	w.StartCol = 3
	redraw(w)
	_, _, style, _ := sim.GetContent(0, 1)
	if style != config.Colorscheme["after"] {
		t.Fatal("horizontal start lost hidden syntax transition")
	}
}

func TestConcealWrappedGutter(t *testing.T) {
	w, sim := concealWindow(t, "c\nabcHIDDEN xyz", 10)
	w.Buf.Settings["ruler"] = true
	w.Buf.Settings["softwrap"] = true
	w.updateDisplayInfo()
	w.Buf.AddConceal(1, 3, 9, "UV", "", "g")
	redraw(w)
	if got := screenRow(sim, 1, 10); got != "2 abcUV xy" {
		t.Fatal(got)
	}
	if got := screenRow(sim, 2, 10); got != "  z" {
		t.Fatal(got)
	}
}

func TestConcealFrameRevealDoesNotLeakAfterDrawing(t *testing.T) {
	w, _ := concealWindow(t, "c\nhidden", 20)
	w.Buf.AddConceal(1, 0, 6, "X", "", "g")
	redraw(w)
	if w.frameReveal != nil {
		t.Fatal("frame snapshot retained")
	}
	w.Buf.GetActiveCursor().Loc = buffer.Loc{Y: 1}
	if w.concealGlyphs(1) != nil {
		t.Fatal("cursor movement used stale reveal state")
	}
	w.Buf.GetActiveCursor().Loc = buffer.Loc{}
	if w.concealGlyphs(1) == nil {
		t.Fatal("conceal failed to resume")
	}
}
