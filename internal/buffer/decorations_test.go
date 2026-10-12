package buffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	lua "github.com/yuin/gopher-lua"
	luar "layeh.com/gopher-luar"
)

func decoBuf() *Buffer {
	return NewBufferFromString("zero\n- [x] one\n- [ ] two\nthree", "", BTDefault)
}

func TestAddConcealClampsAndSorts(t *testing.T) {
	b := decoBuf()
	assert.True(t, b.AddConceal(1, 2, 5, "✔", "special", "cb"))
	assert.True(t, b.AddConceal(1, 0, 2, "", "", "cb"))
	assert.True(t, b.AddConceal(2, 6, 100, "…", "", "cb")) // clamped to line end

	c := b.LineConceals(1)
	assert.Len(t, c, 2)
	assert.Equal(t, 0, c[0].Start)
	assert.Equal(t, 2, c[1].Start)
	assert.Equal(t, "✔", c[1].Replacement)
	assert.Equal(t, 9, b.LineConceals(2)[0].End)
	assert.True(t, b.HasDecorations())
}

func TestAddConcealRejects(t *testing.T) {
	b := decoBuf()
	assert.False(t, b.AddConceal(99, 0, 1, "", "", "g"), "missing line")
	assert.False(t, b.AddConceal(1, 3, 3, "x", "", "g"), "empty range")
	assert.True(t, b.AddConceal(1, 2, 5, "", "", "g"))
	assert.False(t, b.AddConceal(1, 4, 6, "", "", "g"), "overlap: first wins")
	assert.Len(t, b.LineConceals(1), 1)
	assert.Nil(t, b.LineConceals(0), "rejected adds must not create records")
}

func TestConcealCharacterColumns(t *testing.T) {
	// columns count characters, not bytes: "é" is 2 bytes, 1 column
	b := NewBufferFromString("é- [x]", "", BTDefault)
	assert.True(t, b.AddConceal(0, 1, 6, "✔", "", "g"))
	assert.False(t, b.AddConceal(0, 0, 7, "", "", "g"))
	assert.Equal(t, 6, b.LineConceals(0)[0].End)
}

func TestEditDropsConcealsOnlyOnTouchedLines(t *testing.T) {
	b := decoBuf()
	b.AddConceal(1, 2, 5, "✔", "", "cb")
	b.AddConceal(2, 2, 5, "☐", "", "cb")
	b.SetVirtualLines(1, []string{"box"}, "v")

	b.Insert(Loc{6, 1}, "!")
	assert.Nil(t, b.LineConceals(1), "edited line loses its conceals")
	assert.Len(t, b.LineVirtualBlocks(1), 1, "virtual rows stay attached")
	assert.Len(t, b.LineConceals(2), 1, "untouched line keeps its conceals")

	b.Remove(Loc{0, 2}, Loc{1, 2})
	assert.Nil(t, b.LineConceals(2))
}

func TestDecorationsFollowLineInsertAndDelete(t *testing.T) {
	b := decoBuf()
	b.AddConceal(2, 2, 5, "☐", "", "cb")
	b.SetVirtualLines(3, []string{"a", "b"}, "v")

	b.Insert(Loc{0, 0}, "new\nlines\n") // two lines inserted above
	assert.Len(t, b.LineConceals(4), 1)
	assert.Equal(t, "- [ ] two", b.Line(4))
	assert.Len(t, b.LineVirtualBlocks(5), 1)

	b.Remove(Loc{0, 0}, Loc{0, 2}) // remove them again
	assert.Len(t, b.LineConceals(2), 1)
	assert.Len(t, b.LineVirtualBlocks(3), 1)

	b.Remove(Loc{4, 2}, Loc{5, 3}) // delete across the end of line 2 into line 3
	assert.Equal(t, 3, b.LinesNum())
	assert.Nil(t, b.LineVirtualBlocks(2), "decorations of a joined-away line are dropped")
}

func TestSplitKeepsDecorationsOnFirstHalf(t *testing.T) {
	b := decoBuf()
	b.SetVirtualLines(1, []string{"box"}, "v")
	b.ConcealLines(1, 1, "h")
	b.Insert(Loc{3, 1}, "\n")
	assert.Len(t, b.LineVirtualBlocks(1), 1)
	assert.Equal(t, []string{"h"}, b.LineHiddenBy(1))
	assert.Nil(t, b.LineVirtualBlocks(2))
	assert.Nil(t, b.LineHiddenBy(2))
}

func TestSplitAtColumnZeroMovesDecorations(t *testing.T) {
	b := decoBuf()
	b.SetVirtualLines(1, []string{"box"}, "v")
	b.Insert(Loc{0, 1}, "\n")
	assert.Nil(t, b.LineVirtualBlocks(1))
	assert.Len(t, b.LineVirtualBlocks(2), 1)
	assert.Equal(t, "- [x] one", b.Line(2))
}

func TestUndoAlsoDropsConceals(t *testing.T) {
	b := decoBuf()
	b.Insert(Loc{0, 1}, "x")
	b.AddConceal(1, 3, 6, "✔", "", "cb")
	b.Undo()
	assert.Equal(t, "- [x] one", b.Line(1))
	assert.Nil(t, b.LineConceals(1))
}

func TestConcealLines(t *testing.T) {
	b := decoBuf()
	b.ConcealLines(-5, 1, "k")
	b.ConcealLines(1, 99, "k")
	b.ConcealLines(1, 1, "other")
	assert.Equal(t, []string{"k"}, b.LineHiddenBy(0))
	assert.Equal(t, []string{"k", "other"}, b.LineHiddenBy(1))
	assert.Equal(t, []string{"k"}, b.LineHiddenBy(3))
}

func TestVirtualLinesReplaceByGroup(t *testing.T) {
	b := decoBuf()
	b.SetVirtualLines(0, []string{"a", "b"}, "g1")
	b.SetVirtualLines(0, []string{"x"}, "g2")
	b.SetVirtualLines(0, []string{"c\nd"}, "g1")
	v := b.LineVirtualBlocks(0)
	assert.Len(t, v, 2)
	assert.Equal(t, "g2", v[0].Group)
	assert.Equal(t, "c d", v[1].Rows[0].Text)

	b.SetVirtualLines(0, nil, "g2")
	b.SetVirtualLines(0, nil, "g1")
	assert.Nil(t, b.LineVirtualBlocks(0))
}

func TestStyleVirtualLine(t *testing.T) {
	b := decoBuf()
	b.SetVirtualLines(0, []string{"│ card #bug │"}, "k")
	assert.True(t, b.StyleVirtualLine(0, 0, 0, -1, "kanban.border", "k"))
	assert.True(t, b.StyleVirtualLine(0, 0, 7, 11, "kanban.tag", "k"))
	assert.False(t, b.StyleVirtualLine(0, 1, 0, 1, "x", "k"), "no such row")
	assert.False(t, b.StyleVirtualLine(0, 0, 0, 1, "x", "nope"), "no such group")
	assert.False(t, b.StyleVirtualLine(0, 0, 5, 5, "x", "k"), "empty span")
	spans := b.LineVirtualBlocks(0)[0].Rows[0].Spans
	assert.Equal(t, []VirtualSpan{{0, -1, "kanban.border"}, {7, 11, "kanban.tag"}}, spans)

	b.SetVirtualLines(0, []string{"new"}, "k")
	assert.Nil(t, b.LineVirtualBlocks(0)[0].Rows[0].Spans, "replacing rows resets styles")
}

func TestClearDecorationsAndGroups(t *testing.T) {
	b := decoBuf()
	b.AddConceal(1, 2, 5, "✔", "", "cb")
	b.AddConceal(1, 6, 9, "", "", "k")
	b.ConcealLines(1, 2, "k")
	b.SetVirtualLines(2, []string{"board"}, "k")

	assert.ElementsMatch(t, []string{"cb", "k"}, b.LineGroups(1))

	b.ClearDecorations("k")
	assert.Len(t, b.LineConceals(1), 1)
	assert.Empty(t, b.LineHiddenBy(1))
	assert.Nil(t, b.LineVirtualBlocks(2))
	assert.Equal(t, []string{"cb"}, b.LineGroups(1))
	assert.True(t, b.HasDecorations())

	b.ClearDecorations("")
	assert.Nil(t, b.LineConceals(1))
	assert.False(t, b.HasDecorations())
}

func TestRevealMode(t *testing.T) {
	b := decoBuf()
	assert.Equal(t, RevealLine, b.GroupRevealMode("k"))
	assert.True(t, b.SetRevealMode("k", "group"))
	assert.Equal(t, RevealGroup, b.GroupRevealMode("k"))
	assert.False(t, b.SetRevealMode("k", "bogus"))
	b.ClearDecorations("")
	assert.Equal(t, RevealGroup, b.GroupRevealMode("k"), "modes survive clearing")
}

func TestSplitBuffersShareDecorations(t *testing.T) {
	b := decoBuf()
	b2 := NewBufferFromString("", "", BTDefault)
	b2.SharedBuffer = b.SharedBuffer
	b.AddConceal(1, 2, 5, "✔", "", "cb")
	assert.Len(t, b2.LineConceals(1), 1)
}

// The API is meant to be called from Lua plugins through gopher-luar; check
// that Lua values (numbers, strings, a table of strings) convert correctly.
func TestDecorationsFromLua(t *testing.T) {
	b := decoBuf()
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("buf", luar.New(L, b))

	err := L.DoString(`
		ok = buf:AddConceal(1, 2, 5, "✔", "special", "cb")
		buf:SetVirtualLines(0, {"┌──┐", "└──┘"}, "box")
		styled = buf:StyleVirtualLine(0, 1, 0, -1, "kanban.border", "box")
		buf:ConcealLines(2, 3, "k")
		mode = buf:SetRevealMode("k", "group")
		has = buf:HasDecorations()
	`)
	assert.NoError(t, err)
	assert.Equal(t, lua.LTrue, L.GetGlobal("ok"))
	assert.Equal(t, lua.LTrue, L.GetGlobal("styled"))
	assert.Equal(t, lua.LTrue, L.GetGlobal("mode"))
	assert.Equal(t, lua.LTrue, L.GetGlobal("has"))
	assert.Equal(t, "└──┘", b.LineVirtualBlocks(0)[0].Rows[1].Text)
	assert.Equal(t, []string{"k"}, b.LineHiddenBy(3))
}
