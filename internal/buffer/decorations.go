package buffer

import (
	"strings"

	"github.com/micro-editor/micro/v2/internal/util"
)

// Decorations are display-only annotations attached to buffer lines. They
// never change the buffer's text. They are rendered only when the buffer
// option "conceal" is enabled, and plugins create them through the exported
// methods below (callable from Lua as buf:AddConceal(...) etc).
//
// Decorations live on the Line objects of the LineArray, so they move
// together with their line when lines are inserted or removed above them,
// and they are shared by all splits viewing the same file.
//
// All columns are character indices, the same unit as Loc.X.

// RevealMode controls when the decorations of a group are suppressed so the
// raw text is shown instead.
type RevealMode int

const (
	// RevealLine suppresses a group's decorations only on lines that hold a
	// cursor or are touched by a selection. This is the default.
	RevealLine RevealMode = iota
	// RevealGroup suppresses all decorations of the group as soon as a
	// cursor or selection touches any line that carries one of them.
	RevealGroup
)

// A Conceal hides the characters [Start, End) of a line and draws
// Replacement (which may be empty) in their place.
type Conceal struct {
	Start, End  int
	Replacement string
	// Style is a colorscheme group name ("" for the default style)
	Style string
	Group string
}

// A VirtualSpan applies the colorscheme group Style to the characters
// [Start, End) of a virtual row. End == -1 means "to the end of the row".
type VirtualSpan struct {
	Start, End int
	Style      string
}

// A VirtualRow is one display-only row drawn below a buffer line.
type VirtualRow struct {
	Text  string
	Spans []VirtualSpan
}

// A VirtualBlock is the list of virtual rows a single group attached to a line.
type VirtualBlock struct {
	Group string
	Rows  []VirtualRow
}

type lineDeco struct {
	conceals []Conceal // sorted by Start, non-overlapping
	hiddenBy []string  // groups that hide this line
	virtual  []VirtualBlock
}

func (d *lineDeco) empty() bool {
	return len(d.conceals) == 0 && len(d.hiddenBy) == 0 && len(d.virtual) == 0
}

// deco returns the decoration record of line n, creating it if needed
func (la *LineArray) deco(n int) *lineDeco {
	if la.lines[n].deco == nil {
		la.lines[n].deco = &lineDeco{}
	}
	return la.lines[n].deco
}

func (la *LineArray) validLine(n int) bool {
	return n >= 0 && n < len(la.lines)
}

// AddConceal hides the characters [startCol, endCol) of the given line and
// draws replacement in their place, using the colorscheme group style.
// Columns are clamped to the line. It returns false and does nothing if the
// line does not exist, the range is empty, or it overlaps an existing
// conceal on that line (the first conceal added wins).
func (la *LineArray) AddConceal(line, startCol, endCol int, replacement, style, group string) bool {
	la.lock.Lock()
	defer la.lock.Unlock()

	if !la.validLine(line) {
		return false
	}
	n := util.CharacterCount(la.lines[line].data)
	startCol = util.Clamp(startCol, 0, n)
	endCol = util.Clamp(endCol, 0, n)
	if startCol >= endCol {
		return false
	}

	d := la.deco(line)
	i := 0
	for ; i < len(d.conceals); i++ {
		c := d.conceals[i]
		if startCol < c.End && c.Start < endCol {
			if d.empty() {
				la.lines[line].deco = nil
			}
			return false
		}
		if c.Start >= endCol {
			break
		}
	}
	replacement = strings.ReplaceAll(replacement, "\n", " ")
	d.conceals = append(d.conceals, Conceal{})
	copy(d.conceals[i+1:], d.conceals[i:])
	d.conceals[i] = Conceal{startCol, endCol, replacement, style, group}
	la.decoActive = true
	return true
}

// ConcealLines hides the whole lines [startLine, endLine] (inclusive) so
// they take no screen rows. Virtual lines attached to hidden lines are still
// drawn. The range is clamped to the buffer.
func (la *LineArray) ConcealLines(startLine, endLine int, group string) {
	la.lock.Lock()
	defer la.lock.Unlock()

	startLine = util.Max(startLine, 0)
	endLine = util.Min(endLine, len(la.lines)-1)
	for y := startLine; y <= endLine; y++ {
		d := la.deco(y)
		found := false
		for _, g := range d.hiddenBy {
			if g == group {
				found = true
				break
			}
		}
		if !found {
			d.hiddenBy = append(d.hiddenBy, group)
		}
		la.decoActive = true
	}
}

// SetVirtualLines attaches display-only rows below the given line, replacing
// any rows the same group previously attached to it. An empty texts list
// removes the group's rows from the line. Newlines in texts are replaced by
// spaces. Rows use the default style until styled with StyleVirtualLine.
func (la *LineArray) SetVirtualLines(line int, texts []string, group string) {
	la.lock.Lock()
	defer la.lock.Unlock()

	if !la.validLine(line) {
		return
	}
	d := la.deco(line)
	for i := 0; i < len(d.virtual); i++ {
		if d.virtual[i].Group == group {
			d.virtual = append(d.virtual[:i], d.virtual[i+1:]...)
			break
		}
	}
	if len(texts) > 0 {
		rows := make([]VirtualRow, len(texts))
		for i, t := range texts {
			rows[i] = VirtualRow{Text: strings.ReplaceAll(t, "\n", " ")}
		}
		d.virtual = append(d.virtual, VirtualBlock{group, rows})
		la.decoActive = true
	}
	if d.empty() {
		la.lines[line].deco = nil
	}
}

// StyleVirtualLine applies the colorscheme group style to the characters
// [startCol, endCol) of virtual row `row` that `group` attached to `line`.
// endCol == -1 means to the end of the row. Later calls take precedence
// where spans overlap. It returns false if that row does not exist.
func (la *LineArray) StyleVirtualLine(line, row, startCol, endCol int, style, group string) bool {
	la.lock.Lock()
	defer la.lock.Unlock()

	if !la.validLine(line) || la.lines[line].deco == nil {
		return false
	}
	d := la.lines[line].deco
	for i := range d.virtual {
		b := &d.virtual[i]
		if b.Group != group {
			continue
		}
		if row < 0 || row >= len(b.Rows) {
			return false
		}
		if startCol < 0 {
			startCol = 0
		}
		if endCol != -1 && endCol <= startCol {
			return false
		}
		b.Rows[row].Spans = append(b.Rows[row].Spans, VirtualSpan{startCol, endCol, style})
		return true
	}
	return false
}

// SetRevealMode sets the reveal-on-cursor mode of a group: "line" (the
// default) or "group". It returns false for an unknown mode.
func (la *LineArray) SetRevealMode(group, mode string) bool {
	la.lock.Lock()
	defer la.lock.Unlock()

	var m RevealMode
	switch mode {
	case "line":
		m = RevealLine
	case "group":
		m = RevealGroup
	default:
		return false
	}
	if la.revealModes == nil {
		la.revealModes = make(map[string]RevealMode)
	}
	la.revealModes[group] = m
	return true
}

// MaxRevealMargin caps SetRevealMargin. Reveal checks scan this many lines
// around each cursor per frame, so keep it small.
const MaxRevealMargin = 10

// SetRevealMargin makes a "group"-mode group also reveal while a cursor or
// selection is within `lines` lines of any line carrying one of its
// decorations (0, the default, means only on those lines). This keeps, say,
// a rendered block from snapping back the moment the cursor steps off its
// edge. Values are clamped to [0, MaxRevealMargin]; returns false if negative.
func (la *LineArray) SetRevealMargin(group string, lines int) bool {
	la.lock.Lock()
	defer la.lock.Unlock()

	if lines < 0 {
		return false
	}
	if lines > MaxRevealMargin {
		lines = MaxRevealMargin
	}
	if la.revealMargins == nil {
		la.revealMargins = make(map[string]int)
	}
	if lines == 0 {
		delete(la.revealMargins, group)
	} else {
		la.revealMargins[group] = lines
	}
	la.maxRevealMargin = 0
	for _, m := range la.revealMargins {
		if m > la.maxRevealMargin {
			la.maxRevealMargin = m
		}
	}
	return true
}

// ClearDecorations removes every decoration of the given group, or of all
// groups if group is "". Reveal modes are kept.
func (la *LineArray) ClearDecorations(group string) {
	la.lock.Lock()
	defer la.lock.Unlock()

	active := false
	for y := range la.lines {
		d := la.lines[y].deco
		if d == nil {
			continue
		}
		if group == "" {
			la.lines[y].deco = nil
			continue
		}
		d.conceals = filterSlice(d.conceals, func(c Conceal) bool { return c.Group != group })
		d.hiddenBy = filterSlice(d.hiddenBy, func(g string) bool { return g != group })
		d.virtual = filterSlice(d.virtual, func(b VirtualBlock) bool { return b.Group != group })
		if d.empty() {
			la.lines[y].deco = nil
		} else {
			active = true
		}
	}
	la.decoActive = active
}

// HasDecorations reports whether the buffer may hold decorations. It can
// return true after all decorations disappeared through edits, but never
// returns false while some exist, so the display can use it to skip all
// decoration work for ordinary buffers.
func (la *LineArray) HasDecorations() bool {
	return la.decoActive
}

// Read accessors, used by the display code. The returned slices must be
// treated as read-only.

// LineConceals returns the conceals of a line, sorted by start column
func (la *LineArray) LineConceals(line int) []Conceal {
	if !la.validLine(line) || la.lines[line].deco == nil {
		return nil
	}
	return la.lines[line].deco.conceals
}

// LineHiddenBy returns the groups that hide the given line
func (la *LineArray) LineHiddenBy(line int) []string {
	if !la.validLine(line) || la.lines[line].deco == nil {
		return nil
	}
	return la.lines[line].deco.hiddenBy
}

// LineVirtualBlocks returns the virtual rows attached to a line, by group,
// in the order the groups first attached them
func (la *LineArray) LineVirtualBlocks(line int) []VirtualBlock {
	if !la.validLine(line) || la.lines[line].deco == nil {
		return nil
	}
	return la.lines[line].deco.virtual
}

// LineGroups returns every group that has any decoration on the given line
func (la *LineArray) LineGroups(line int) []string {
	if !la.validLine(line) || la.lines[line].deco == nil {
		return nil
	}
	d := la.lines[line].deco
	var groups []string
	add := func(g string) {
		for _, h := range groups {
			if h == g {
				return
			}
		}
		groups = append(groups, g)
	}
	for _, c := range d.conceals {
		add(c.Group)
	}
	for _, g := range d.hiddenBy {
		add(g)
	}
	for _, b := range d.virtual {
		add(b.Group)
	}
	return groups
}

// GroupRevealMode returns the reveal mode of a group
func (la *LineArray) GroupRevealMode(group string) RevealMode {
	return la.revealModes[group]
}

// GroupRevealMargin returns a group's reveal margin in lines (0 if unset)
func (la *LineArray) GroupRevealMargin(group string) int {
	return la.revealMargins[group]
}

// RevealMarginMax returns the largest reveal margin of any group, so the
// display can skip the margin scan entirely when it is 0.
func (la *LineArray) RevealMarginMax() int {
	return la.maxRevealMargin
}

// dropConceals removes the conceals of lines [y1, y2]; called after an edit
// touched those lines, because their column ranges can no longer be trusted.
// Hidden-line flags and virtual rows stay attached.
func (la *LineArray) dropConceals(y1, y2 int) {
	if !la.decoActive {
		return
	}
	y1 = util.Max(y1, 0)
	y2 = util.Min(y2, len(la.lines)-1)
	for y := y1; y <= y2; y++ {
		d := la.lines[y].deco
		if d == nil {
			continue
		}
		d.conceals = nil
		if d.empty() {
			la.lines[y].deco = nil
		}
	}
}

func filterSlice[T any](s []T, keep func(T) bool) []T {
	out := s[:0]
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}
