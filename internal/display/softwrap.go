package display

import (
	runewidth "github.com/mattn/go-runewidth"
	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/internal/util"
)

// SLoc represents a vertical scrolling location, i.e. a location of a visual line
// in the buffer. When softwrap is enabled, a buffer line may be displayed as
// multiple visual lines (rows). So SLoc stores a number of a line in the buffer
// and a number of a row within this line.
type SLoc struct {
	Line, Row int
}

// LessThan returns true if s is less b
func (s SLoc) LessThan(b SLoc) bool {
	if s.Line < b.Line {
		return true
	}
	return s.Line == b.Line && s.Row < b.Row
}

// GreaterThan returns true if s is bigger than b
func (s SLoc) GreaterThan(b SLoc) bool {
	if s.Line > b.Line {
		return true
	}
	return s.Line == b.Line && s.Row > b.Row
}

// LessEqual returns true if s is less than or equal to b
func (s SLoc) LessEqual(b SLoc) bool {
	if s.Line < b.Line {
		return true
	}
	if s.Line == b.Line && s.Row < b.Row {
		return true
	}
	return s == b
}

// GreaterEqual returns true if s is bigger than or equal to b
func (s SLoc) GreaterEqual(b SLoc) bool {
	if s.Line > b.Line {
		return true
	}
	if s.Line == b.Line && s.Row > b.Row {
		return true
	}
	return s == b
}

// VLoc represents a location in the buffer as a visual location in the
// linewrapped buffer.
type VLoc struct {
	SLoc
	VisualX int
}

type SoftWrap interface {
	Scroll(s SLoc, n int) SLoc
	Diff(s1, s2 SLoc) int
	SLocFromLoc(loc buffer.Loc) SLoc
	VLocFromLoc(loc buffer.Loc) VLoc
	LocFromVLoc(vloc VLoc) buffer.Loc
}

func (w *BufWindow) getVLocFromLoc(loc buffer.Loc) VLoc {
	if glyphs := w.concealGlyphs(loc.Y); glyphs != nil {
		return w.concealedVLoc(loc, glyphs, true)
	}
	vloc := VLoc{SLoc: SLoc{loc.Y, 0}, VisualX: 0}

	if loc.X <= 0 {
		return vloc
	}

	if w.bufWidth <= 0 {
		return vloc
	}

	wordwrap := w.Buf.Settings["wordwrap"].(bool)
	tabsize := util.IntOpt(w.Buf.Settings["tabsize"])

	line := w.Buf.LineBytes(loc.Y)
	x := 0
	totalwidth := 0

	wordwidth := 0
	wordoffset := 0

	for len(line) > 0 {
		r, _, size := util.DecodeCharacter(line)
		line = line[size:]

		width := 0
		switch r {
		case '\t':
			ts := tabsize - (totalwidth % tabsize)
			width = util.Min(ts, w.bufWidth-vloc.VisualX)
			totalwidth += ts
		default:
			width = runewidth.RuneWidth(r)
			totalwidth += width
		}

		wordwidth += width

		// Collect a complete word to know its width.
		// If wordwrap is off, every single character is a complete "word".
		if wordwrap {
			if !util.IsWhitespace(r) && len(line) > 0 && wordwidth < w.bufWidth {
				if x < loc.X {
					wordoffset += width
					x++
				}
				continue
			}
		}

		// If a word (or just a wide rune) does not fit in the window
		if vloc.VisualX+wordwidth > w.bufWidth && vloc.VisualX > 0 {
			vloc.Row++
			vloc.VisualX = 0
		}

		if x == loc.X {
			vloc.VisualX += wordoffset
			return vloc
		}
		x++

		vloc.VisualX += wordwidth

		wordwidth = 0
		wordoffset = 0

		if vloc.VisualX >= w.bufWidth {
			vloc.Row++
			vloc.VisualX = 0
		}
	}
	return vloc
}

func (w *BufWindow) getLocFromVLoc(svloc VLoc) buffer.Loc {
	if glyphs := w.concealGlyphs(svloc.Line); glyphs != nil {
		return w.concealedLoc(svloc, glyphs)
	}
	loc := buffer.Loc{X: 0, Y: svloc.Line}

	if w.bufWidth <= 0 {
		return loc
	}

	wordwrap := w.Buf.Settings["wordwrap"].(bool)
	tabsize := util.IntOpt(w.Buf.Settings["tabsize"])

	line := w.Buf.LineBytes(svloc.Line)
	vloc := VLoc{SLoc: SLoc{svloc.Line, 0}, VisualX: 0}

	totalwidth := 0

	var widths []int
	if wordwrap {
		widths = make([]int, 0, w.bufWidth)
	} else {
		widths = make([]int, 0, 1)
	}
	wordwidth := 0

	for len(line) > 0 {
		r, _, size := util.DecodeCharacter(line)
		line = line[size:]

		width := 0
		switch r {
		case '\t':
			ts := tabsize - (totalwidth % tabsize)
			width = util.Min(ts, w.bufWidth-vloc.VisualX)
			totalwidth += ts
		default:
			width = runewidth.RuneWidth(r)
			totalwidth += width
		}

		widths = append(widths, width)
		wordwidth += width

		// Collect a complete word to know its width.
		// If wordwrap is off, every single character is a complete "word".
		if wordwrap {
			if !util.IsWhitespace(r) && len(line) > 0 && wordwidth < w.bufWidth {
				continue
			}
		}

		// If a word (or just a wide rune) does not fit in the window
		if vloc.VisualX+wordwidth > w.bufWidth && vloc.VisualX > 0 {
			if vloc.Row == svloc.Row {
				if wordwrap {
					// it's a word, not a wide rune
					loc.X--
				}
				return loc
			}
			vloc.Row++
			vloc.VisualX = 0
		}

		for i := range widths {
			vloc.VisualX += widths[i]
			if vloc.Row == svloc.Row && vloc.VisualX > svloc.VisualX {
				return loc
			}
			loc.X++
		}

		widths = widths[:0]
		wordwidth = 0

		if vloc.VisualX >= w.bufWidth {
			vloc.Row++
			vloc.VisualX = 0
		}
	}
	return loc
}

// beginRowModel shares one reveal snapshot throughout an operation. Drawing may
// already own the snapshot; nested mapping calls must not clear that snapshot.
func (w *BufWindow) beginRowModel() func() {
	if w.frameReveal != nil || !w.decorationsEnabled() {
		return func() {}
	}
	w.frameReveal = w.computeDecorationReveal()
	return func() { w.frameReveal = nil }
}

// lineHidden and the two row counts form T4's drawing contract. Text rows come
// first, followed by unwrapped virtual rows in block/insertion order.
func (w *BufWindow) lineHidden(line int) bool {
	if !w.decorationsEnabled() {
		return false
	}
	reveal := w.decorationRevealState()
	for _, group := range w.Buf.LineHiddenBy(line) {
		if !reveal.suppressed(line, group) {
			return true
		}
	}
	return false
}

func (w *BufWindow) getTextRowCount(line int) int {
	if line < 0 || line >= w.Buf.LinesNum() || w.lineHidden(line) {
		return 0
	}
	if !w.Buf.Settings["softwrap"].(bool) {
		return 1
	}
	eol := buffer.Loc{X: util.CharacterCount(w.Buf.LineBytes(line)), Y: line}
	return w.getVLocFromLoc(eol).Row + 1
}

func (w *BufWindow) getVirtualRowCount(line int) int {
	if !w.decorationsEnabled() {
		return 0
	}
	reveal := w.decorationRevealState()
	count := 0
	for _, block := range w.Buf.LineVirtualBlocks(line) {
		if !reveal.suppressed(line, block.Group) {
			count += len(block.Rows)
		}
	}
	return count
}

func (w *BufWindow) getRowCount(line int) int {
	return w.getTextRowCount(line) + w.getVirtualRowCount(line)
}

// normalizeSLoc clamps stale rows after edits/reveal and skips zero-row lines.
// Prefer the following visible row; at a hidden tail use the last preceding row.
// If every line has zero rows, {0,0} is an empty-view sentinel.
func (w *BufWindow) normalizeSLoc(s SLoc) SLoc {
	s.Line = util.Clamp(s.Line, 0, w.Buf.LinesNum()-1)
	if count := w.getRowCount(s.Line); count > 0 {
		s.Row = util.Clamp(s.Row, 0, count-1)
		return s
	}
	for line := s.Line + 1; line < w.Buf.LinesNum(); line++ {
		if w.getRowCount(line) > 0 {
			return SLoc{line, 0}
		}
	}
	for line := s.Line - 1; line >= 0; line-- {
		if count := w.getRowCount(line); count > 0 {
			return SLoc{line, count - 1}
		}
	}
	return SLoc{}
}

func (w *BufWindow) scrollUp(s SLoc, n int) SLoc {
	for n > 0 {
		if n <= s.Row {
			s.Row -= n
			break
		}
		previous := s.Line - 1
		for previous >= 0 && w.getRowCount(previous) == 0 {
			previous--
		}
		if previous < 0 {
			s.Row = 0
			break
		}
		n -= s.Row + 1
		s = SLoc{previous, w.getRowCount(previous) - 1}
	}
	return s
}

func (w *BufWindow) scrollDown(s SLoc, n int) SLoc {
	for n > 0 {
		count := w.getRowCount(s.Line)
		if n < count-s.Row {
			s.Row += n
			break
		}
		next := s.Line + 1
		for next < w.Buf.LinesNum() && w.getRowCount(next) == 0 {
			next++
		}
		if next >= w.Buf.LinesNum() {
			s.Row = util.Max(0, count-1)
			break
		}
		n -= count - s.Row
		s = SLoc{next, 0}
	}
	return s
}

func (w *BufWindow) scroll(s SLoc, n int) SLoc {
	s = w.normalizeSLoc(s)
	if w.getRowCount(s.Line) == 0 {
		return s
	}
	if n < 0 {
		// Avoid negating MinInt, whose magnitude is not representable as int.
		s = w.scrollUp(s, -(n + 1))
		return w.scrollUp(s, 1)
	}
	return w.scrollDown(s, n)
}

func (w *BufWindow) diff(s1, s2 SLoc) int {
	n := 0
	for s1.LessThan(s2) {
		if s1.Line < s2.Line {
			n += w.getRowCount(s1.Line) - s1.Row
			s1.Line++
			s1.Row = 0
		} else {
			n += s2.Row - s1.Row
			s1.Row = s2.Row
		}
	}
	return n
}

// Scroll returns the location which is n visual lines below the location s
// i.e. the result of scrolling n lines down. n can be negative,
// which means scrolling up. The returned location is guaranteed to be
// within the buffer boundaries.
func (w *BufWindow) Scroll(s SLoc, n int) SLoc {
	if !w.Buf.Settings["softwrap"].(bool) && !w.decorationsEnabled() {
		// Drop stale virtual-row offsets when decorations have been disabled.
		s.Line = util.Clamp(s.Line, 0, w.Buf.LinesNum()-1)
		s.Row = 0
		// Clamp before addition to avoid overflow on large scroll requests.
		n = util.Clamp(n, -s.Line, w.Buf.LinesNum()-1-s.Line)
		s.Line += n
		return s
	}
	done := w.beginRowModel()
	defer done()
	return w.scroll(s, n)
}

// Diff returns the signed visual-row distance between two scrolling locations.
func (w *BufWindow) Diff(s1, s2 SLoc) int {
	if !w.Buf.Settings["softwrap"].(bool) && !w.decorationsEnabled() {
		return s2.Line - s1.Line
	}
	done := w.beginRowModel()
	defer done()
	s1, s2 = w.normalizeSLoc(s1), w.normalizeSLoc(s2)
	if s1.GreaterThan(s2) {
		return -w.diff(s2, s1)
	}
	return w.diff(s1, s2)
}

// SLocFromLoc returns the visual row containing a source location. Hidden source
// lines project onto the next visible row, or the preceding row at EOF.
func (w *BufWindow) SLocFromLoc(loc buffer.Loc) SLoc {
	// Keep the uniform-row lookup O(1), without scanning the source line.
	if !w.Buf.Settings["softwrap"].(bool) && !w.decorationsEnabled() {
		return SLoc{loc.Y, 0}
	}
	return w.VLocFromLoc(loc).SLoc
}

// VLocFromLoc maps source character coordinates to displayed coordinates.
func (w *BufWindow) VLocFromLoc(loc buffer.Loc) VLoc {
	done := w.beginRowModel()
	defer done()
	if w.lineHidden(loc.Y) {
		return VLoc{SLoc: w.normalizeSLoc(SLoc{loc.Y, 0})}
	}
	if !w.Buf.Settings["softwrap"].(bool) {
		if glyphs := w.concealGlyphs(loc.Y); glyphs != nil {
			return w.concealedVLoc(loc, glyphs, false)
		}
		tabsize := util.IntOpt(w.Buf.Settings["tabsize"])
		visualx := util.StringWidth(w.Buf.LineBytes(loc.Y), loc.X, tabsize)
		return VLoc{SLoc{loc.Y, 0}, visualx}
	}
	return w.getVLocFromLoc(loc)
}

// LocFromVLoc maps displayed coordinates back to source characters. Every
// virtual row maps to the end of its anchor line, regardless of horizontal x.
func (w *BufWindow) LocFromVLoc(vloc VLoc) buffer.Loc {
	done := w.beginRowModel()
	defer done()
	if w.decorationsEnabled() {
		vloc.SLoc = w.normalizeSLoc(vloc.SLoc)
		if vloc.Row >= w.getTextRowCount(vloc.Line) {
			return buffer.Loc{X: util.CharacterCount(w.Buf.LineBytes(vloc.Line)), Y: vloc.Line}
		}
	}
	if !w.Buf.Settings["softwrap"].(bool) {
		if loc, ok := w.concealLocFromVisual(vloc.Line, vloc.VisualX); ok {
			return loc
		}
		tabsize := util.IntOpt(w.Buf.Settings["tabsize"])
		x := util.GetCharPosInLine(w.Buf.LineBytes(vloc.Line), vloc.VisualX, tabsize)
		return buffer.Loc{x, vloc.Line}
	}
	return w.getLocFromVLoc(vloc)
}

// placedGlyph is a displayed character after applying the same tab and word
// wrapping rules as displayBuffer. Source coordinates remain on displayGlyph.
type placedGlyph struct {
	glyph         displayGlyph
	row, x, width int
}

func (w *BufWindow) placeConcealGlyphs(line int, glyphs []displayGlyph, wrap bool) ([]placedGlyph, VLoc) {
	end := VLoc{SLoc: SLoc{line, 0}}
	if wrap && w.bufWidth <= 0 {
		return nil, end
	}
	placed := make([]placedGlyph, 0, len(glyphs))
	wordStart, wordWidth := 0, 0
	widths := make([]int, 0)
	wordwrap := wrap && w.Buf.Settings["wordwrap"].(bool)
	for i, g := range glyphs {
		width := g.width
		if wrap && g.r == '\t' {
			width = util.Min(width, w.bufWidth-end.VisualX)
		}
		widths = append(widths, width)
		wordWidth += width
		if wordwrap && !util.IsWhitespace(g.r) && i+1 < len(glyphs) && wordWidth < w.bufWidth {
			continue
		}
		if wrap && end.VisualX+wordWidth > w.bufWidth && end.VisualX > 0 {
			end.Row++
			end.VisualX = 0
		}
		for j, width := range widths {
			placed = append(placed, placedGlyph{glyphs[wordStart+j], end.Row, end.VisualX, width})
			end.VisualX += width
		}
		if wrap && end.VisualX >= w.bufWidth {
			end.Row++
			end.VisualX = 0
		}
		wordStart = i + 1
		wordWidth = 0
		widths = widths[:0]
	}
	return placed, end
}

func (w *BufWindow) concealedVLoc(loc buffer.Loc, glyphs []displayGlyph, wrap bool) VLoc {
	placed, end := w.placeConcealGlyphs(loc.Y, glyphs, wrap)
	for _, p := range placed {
		if loc.X <= p.glyph.col || (p.glyph.synthetic && loc.X < p.glyph.end) {
			return VLoc{SLoc{loc.Y, p.row}, p.x}
		}
	}
	return end
}

func (w *BufWindow) concealedLoc(vloc VLoc, glyphs []displayGlyph) buffer.Loc {
	placed, end := w.placeConcealGlyphs(vloc.Line, glyphs, true)
	eol := buffer.Loc{X: util.CharacterCount(w.Buf.LineBytes(vloc.Line)), Y: vloc.Line}
	if w.bufWidth <= 0 {
		return buffer.Loc{Y: vloc.Line}
	}
	previous := -1
	for i, p := range placed {
		if p.row > vloc.Row {
			if previous >= 0 {
				return buffer.Loc{X: placed[previous].glyph.col, Y: vloc.Line}
			}
			return buffer.Loc{X: p.glyph.col, Y: vloc.Line}
		}
		if p.row == vloc.Row {
			if p.x+p.width > vloc.VisualX {
				return buffer.Loc{X: p.glyph.col, Y: vloc.Line}
			}
			previous = i
		}
	}
	if vloc.Row < end.Row && previous >= 0 {
		return buffer.Loc{X: placed[previous].glyph.col, Y: vloc.Line}
	}
	return eol
}

// CursorScroll moves by visual rows, then skips synthetic rows in the direction
// of travel. At an edge containing only virtual rows, stay on the last editable
// row. Mouse mapping intentionally keeps Scroll's virtual-row behavior.
func (w *BufWindow) CursorScroll(s SLoc, n int) SLoc {
	done := w.beginRowModel()
	defer done()
	target := w.Scroll(s, n)
	if n == 0 || target.Row < w.getTextRowCount(target.Line) {
		return target
	}
	if n < 0 {
		for line := target.Line; line >= 0; line-- {
			if count := w.getTextRowCount(line); count > 0 {
				return SLoc{line, count - 1}
			}
		}
	} else {
		for line := target.Line + 1; line < w.Buf.LinesNum(); line++ {
			if w.getTextRowCount(line) > 0 {
				return SLoc{line, 0}
			}
		}
	}
	// No editable row in that direction: find the nearest boundary text row.
	if n > 0 {
		for line := target.Line; line >= 0; line-- {
			if count := w.getTextRowCount(line); count > 0 {
				return SLoc{line, count - 1}
			}
		}
	} else {
		for line := target.Line; line < w.Buf.LinesNum(); line++ {
			if w.getTextRowCount(line) > 0 {
				return SLoc{line, 0}
			}
		}
	}
	return s
}
