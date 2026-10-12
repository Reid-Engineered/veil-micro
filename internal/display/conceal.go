package display

import (
	"github.com/mattn/go-runewidth"
	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/internal/config"
	"github.com/micro-editor/micro/v2/internal/util"
	"github.com/micro-editor/tcell/v2"
)

// decorationReveal is local to a window, never shared between buffer splits.
// T3/T4 can use the same suppression decision for hidden and virtual rows.
type decorationReveal struct {
	lines  map[int]bool
	groups map[string]bool
}

func (w *BufWindow) decorationsEnabled() bool {
	enabled, _ := w.Buf.Settings["conceal"].(bool)
	return enabled && w.Buf.HasDecorations()
}

func (w *BufWindow) computeDecorationReveal() *decorationReveal {
	if !w.decorationsEnabled() {
		return nil
	}
	result := &decorationReveal{lines: make(map[int]bool), groups: make(map[string]bool)}
	for _, c := range w.Buf.GetCursors() {
		result.lines[c.Y] = true
		if c.HasSelection() {
			start, end := c.CurSelection[0], c.CurSelection[1]
			if start.GreaterThan(end) {
				start, end = end, start
			}
			last := end.Y
			if end.X == 0 {
				last--
			}
			for line := util.Max(0, start.Y); line <= util.Min(last, w.Buf.LinesNum()-1); line++ {
				result.lines[line] = true
			}
		}
	}
	for line := range result.lines {
		for _, group := range w.Buf.LineGroups(line) {
			if w.Buf.GroupRevealMode(group) == buffer.RevealGroup {
				result.groups[group] = true
			}
		}
	}
	return result
}

func (w *BufWindow) decorationRevealState() *decorationReveal {
	if w.frameReveal != nil {
		return w.frameReveal
	}
	return w.computeDecorationReveal()
}

func (reveal *decorationReveal) suppressed(line int, group string) bool {
	return reveal == nil || reveal.lines[line] || reveal.groups[group]
}

// decorationSuppressed is reserved for T3/T4 to apply the same window-local
// reveal decision to hidden lines and virtual rows as T2 applies to conceals.
func (w *BufWindow) decorationSuppressed(line int, group string) bool {
	return w.decorationRevealState().suppressed(line, group)
}

// displayGlyph retains source character coordinates while describing a display
// character. Synthetic glyphs belong to a replacement, not selectable buffer text.
// VisualX and Width count cells before wrapping; tabs use the displayed tab stop.
// T3 can consume this same sequence for conceal-aware wrapping and hit testing.
type displayGlyph struct {
	r              rune
	combc          []rune
	col, end       int
	visualX, width int
	synthetic      bool
	style          string
}

func displayRuneWidth(r rune, visualX, tabsize int) int {
	if r == '\t' {
		return tabsize - visualX%tabsize
	}
	return runewidth.RuneWidth(r)
}

// concealGlyphs returns nil for the original fast path, and a non-nil (possibly
// empty) sequence when a conceal changes this line's displayed text.
func (w *BufWindow) concealGlyphs(line int) []displayGlyph {
	if !w.decorationsEnabled() {
		return nil
	}
	reveal := w.decorationRevealState()
	conceals := w.Buf.LineConceals(line)
	active := false
	for _, c := range conceals {
		if !reveal.suppressed(line, c.Group) {
			active = true
			break
		}
	}
	if !active {
		return nil
	}
	glyphs := make([]displayGlyph, 0)
	data := w.Buf.LineBytes(line)
	col, visualX, index := 0, 0, 0
	tabsize := util.IntOpt(w.Buf.Settings["tabsize"])
	for len(data) > 0 {
		for index < len(conceals) && conceals[index].End <= col {
			index++
		}
		if index < len(conceals) && conceals[index].Start == col && !reveal.suppressed(line, conceals[index].Group) {
			c := conceals[index]
			for col < c.End && len(data) > 0 {
				_, _, size := util.DecodeCharacter(data)
				data = data[size:]
				col++
			}
			replacement := []byte(c.Replacement)
			for len(replacement) > 0 {
				ch, combc, size := util.DecodeCharacter(replacement)
				replacement = replacement[size:]
				width := displayRuneWidth(ch, visualX, tabsize)
				glyphs = append(glyphs, displayGlyph{ch, combc, c.Start, c.End, visualX, width, true, c.Style})
				visualX += width
			}
			continue
		}
		ch, combc, size := util.DecodeCharacter(data)
		data = data[size:]
		width := displayRuneWidth(ch, visualX, tabsize)
		glyphs = append(glyphs, displayGlyph{ch, combc, col, col + 1, visualX, width, false, ""})
		visualX += width
		col++
	}
	return glyphs
}

func displayGlyphStart(glyphs []displayGlyph, visualX int) int {
	for i, g := range glyphs {
		if g.visualX+g.width > visualX {
			return i
		}
	}
	return len(glyphs)
}

func (w *BufWindow) concealStartInfo(n, line int, glyphs []displayGlyph) ([]byte, int, int, *tcell.Style) {
	index := displayGlyphStart(glyphs, n)
	col, visualX := util.CharacterCount(w.Buf.LineBytes(line)), 0
	if index < len(glyphs) {
		col, visualX = glyphs[index].col, glyphs[index].visualX
	} else if len(glyphs) > 0 {
		g := glyphs[len(glyphs)-1]
		visualX = g.visualX + g.width
	}
	style := config.DefStyle
	var changed *tcell.Style
	for x := 0; x <= col; x++ {
		var found bool
		style, found = w.getStyle(style, buffer.Loc{X: x, Y: line})
		if found {
			copy := style
			changed = &copy
		}
	}
	return util.SliceEnd(w.Buf.LineBytes(line), col), n - visualX, col, changed
}

// concealLocFromVisual maps replacement cells to the start of their source range.
// Clicking then reveals that line, so a replacement can never trap the cursor.
func (w *BufWindow) concealLocFromVisual(line, visualX int) (buffer.Loc, bool) {
	glyphs := w.concealGlyphs(line)
	if glyphs == nil {
		return buffer.Loc{}, false
	}
	index := displayGlyphStart(glyphs, visualX)
	col := util.CharacterCount(w.Buf.LineBytes(line))
	if index < len(glyphs) {
		col = glyphs[index].col
	}
	return buffer.Loc{X: col, Y: line}, true
}

func concealStyle(name string) tcell.Style {
	if style, ok := config.Colorscheme[name]; ok && name != "" {
		return style
	}
	return config.DefStyle
}
