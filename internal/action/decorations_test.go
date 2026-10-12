package action

import (
	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/internal/config"
	"github.com/micro-editor/micro/v2/internal/display"
	ulua "github.com/micro-editor/micro/v2/internal/lua"
	"github.com/micro-editor/micro/v2/internal/screen"
	"github.com/micro-editor/tcell/v2"
	lua "github.com/yuin/gopher-lua"
	"testing"
)

func decorationPane(t *testing.T, text string, wrap bool) *BufPane {
	t.Helper()
	oldSettings, oldDir := config.GlobalSettings, config.ConfigDir
	oldLua, oldScreen, oldColors, oldStyle := ulua.L, screen.Screen, config.Colorscheme, config.DefStyle
	config.InitRuntimeFiles(false)
	if err := config.InitGlobalSettings(); err != nil {
		t.Fatal(err)
	}
	config.ConfigDir = t.TempDir()
	for _, key := range []string{"syntax", "savecursor", "saveundo", "backup", "fakecursor"} {
		config.GlobalSettings[key] = false
	}
	ulua.L = lua.NewState()
	config.Colorscheme = make(map[string]tcell.Style)
	config.DefStyle = tcell.StyleDefault
	sim, err := screen.InitSimScreen()
	if err != nil {
		t.Fatal(err)
	}
	sim.SetSize(30, 20)
	b := buffer.NewBufferFromString(text, "", buffer.BTDefault)
	b.Settings["conceal"] = true
	b.Settings["softwrap"] = wrap
	b.Settings["wordwrap"] = false
	b.Settings["ruler"] = false
	b.Settings["scrollbar"] = false
	b.Settings["statusline"] = true
	b.Settings["scrollmargin"] = float64(0)
	b.Settings["matchbrace"] = false
	w := display.NewBufWindow(0, 0, 8, 6, b)
	w.Resize(8, 6)
	h := newBufPane(b, w, nil)
	t.Cleanup(func() {
		b.Close()
		ulua.L.Close()
		sim.Fini()
		config.GlobalSettings, config.ConfigDir = oldSettings, oldDir
		ulua.L, screen.Screen, config.Colorscheme, config.DefStyle = oldLua, oldScreen, oldColors, oldStyle
	})
	return h
}

func TestDecoratedVerticalActions(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "wrapped"}[wrap], func(t *testing.T) {
			h := decorationPane(t, "start\nanchor\nhidden\nnext\ntail", wrap)
			h.Buf.ConcealLines(2, 2, "h")
			h.Buf.ConcealLines(4, 4, "h")
			h.Buf.SetVirtualLines(1, []string{"v1", "v2"}, "v")
			h.Buf.SetVirtualLines(4, []string{"tail1", "tail2"}, "v")
			h.Cursor.GotoLoc(buffer.Loc{X: 2, Y: 1})
			h.MoveCursorDown(1)
			if h.Cursor.Loc != (buffer.Loc{X: 2, Y: 3}) {
				t.Fatal("down", h.Cursor.Loc)
			}
			h.MoveCursorUp(1)
			if h.Cursor.Loc != (buffer.Loc{X: 2, Y: 1}) {
				t.Fatal("up", h.Cursor.Loc)
			}
			h.MoveCursorDown(1)
			h.MoveCursorDown(1)
			if h.Cursor.Loc != (buffer.Loc{X: 4, Y: 3}) {
				t.Fatal("hidden EOF", h.Cursor.Loc)
			}
			h.MoveCursorDown(1)
			if h.Cursor.Y != 3 {
				t.Fatal("moved into hidden EOF")
			}
			h.Cursor.GotoLoc(buffer.Loc{X: 2})
			h.MoveCursorDown(3)
			if h.Cursor.Y != 3 {
				t.Fatal("page into virtual row", h.Cursor.Loc)
			}
			h.MoveCursorUp(10)
			if h.Cursor.Y != 0 {
				t.Fatal(h.Cursor.Loc)
			}
			h.MoveCursorUp(1)
			if h.Cursor.Loc != (buffer.Loc{}) {
				t.Fatal(h.Cursor.Loc)
			}
			h.Cursor.GotoLoc(buffer.Loc{X: 2, Y: 1})
			h.MoveCursorDown(0)
			if h.Cursor.Loc != (buffer.Loc{X: 2, Y: 1}) {
				t.Fatal("zero movement")
			}
		})
	}
}

func TestDecoratedWrappedRowActions(t *testing.T) {
	h := decorationPane(t, "abcdefghijkl\nanchor\nlast", true)
	h.Buf.SetVirtualLines(1, []string{"v1", "v2"}, "v")
	h.Cursor.GotoLoc(buffer.Loc{X: 2})
	h.MoveCursorDown(1)
	if h.Cursor.Loc != (buffer.Loc{X: 10}) {
		t.Fatal(h.Cursor.Loc)
	}
	h.MoveCursorDown(1)
	if h.Cursor.Loc != (buffer.Loc{X: 2, Y: 1}) {
		t.Fatal(h.Cursor.Loc)
	}
	h.MoveCursorDown(1)
	if h.Cursor.Loc != (buffer.Loc{X: 2, Y: 2}) {
		t.Fatal(h.Cursor.Loc)
	}
	h.MoveCursorUp(1)
	if h.Cursor.Loc != (buffer.Loc{X: 2, Y: 1}) {
		t.Fatal(h.Cursor.Loc)
	}
}

func TestDecoratedMouseRevealAndDrag(t *testing.T) {
	h := decorationPane(t, "start\nraw1\nraw2\nlast", false)
	h.Buf.ConcealLines(1, 2, "board")
	h.Buf.SetVirtualLines(1, []string{"board1", "board2"}, "board")
	h.Buf.SetRevealMode("board", "group")
	h.BWindow.Clear()
	h.BWindow.Display()
	if !h.MousePress(tcell.NewEventMouse(2, 2, tcell.Button1, tcell.ModNone, "")) {
		t.Fatal("click rejected")
	}
	if h.Cursor.Loc != (buffer.Loc{X: 4, Y: 1}) {
		t.Fatal("virtual click", h.Cursor.Loc)
	}
	h.BWindow.Clear()
	h.BWindow.Display()
	// Source is now revealed; dragging selects actual source lines.
	if !h.MouseDrag(tcell.NewEventMouse(2, 2, tcell.Button1, tcell.ModNone, "")) || h.Cursor.Loc != (buffer.Loc{X: 2, Y: 2}) || !h.Cursor.HasSelection() {
		t.Fatal("drag", h.Cursor.Loc)
	}
	if h.MousePress(tcell.NewEventMouse(0, 5, tcell.Button1, tcell.ModNone, "")) {
		t.Fatal("statusline click accepted")
	}
}

func TestDecoratedScrollingIncludesTrailingVirtualRows(t *testing.T) {
	h := decorationPane(t, "cursor\nanchor", false)
	h.Buf.SetVirtualLines(1, []string{"1", "2", "3", "4", "5", "6", "7", "8"}, "v")
	if h.ScrollReachedEnd() {
		t.Fatal("virtual EOF falsely reached")
	}
	h.ScrollDown(7)
	h.ScrollAdjust()
	if got := h.GetView().StartLine; got != (display.SLoc{Line: 1, Row: 4}) {
		t.Fatal(got)
	}
	if !h.ScrollReachedEnd() {
		t.Fatal("virtual EOF not reached")
	}
}

func TestDisabledDecorationsPreserveOrdinaryMovement(t *testing.T) {
	h := decorationPane(t, "start\nanchor\nnext", false)
	h.Buf.SetVirtualLines(1, []string{"v"}, "v")
	h.Buf.ConcealLines(1, 1, "h")
	h.Buf.Settings["conceal"] = false
	h.Cursor.GotoLoc(buffer.Loc{X: 2})
	h.MoveCursorDown(1)
	if h.Cursor.Loc != (buffer.Loc{X: 2, Y: 1}) {
		t.Fatal(h.Cursor.Loc)
	}
}

func TestDecoratedKeyboardSelectionRevealsSource(t *testing.T) {
	h := decorationPane(t, "start\nhidden\nnext", true)
	h.Buf.ConcealLines(1, 1, "h")
	h.Buf.SetVirtualLines(1, []string{"v1", "v2"}, "v")
	h.Cursor.GotoLoc(buffer.Loc{X: 2})
	h.SelectDown()
	if h.Cursor.Loc != (buffer.Loc{X: 2, Y: 2}) || !h.Cursor.HasSelection() {
		t.Fatal(h.Cursor.Loc)
	}
	// The selected hidden source is now revealed and editable on the return path.
	h.SelectUp()
	if h.Cursor.Loc != (buffer.Loc{X: 2, Y: 1}) {
		t.Fatal(h.Cursor.Loc)
	}
}
