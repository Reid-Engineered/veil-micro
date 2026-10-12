package spec

import (
	"fmt"
	"os"
	"testing"

	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/internal/config"
	ulua "github.com/micro-editor/micro/v2/internal/lua"
	lua "github.com/yuin/gopher-lua"
	luar "layeh.com/gopher-luar"
)

func init() {
	// Same setup as internal/buffer's tests: buffers need settings and a Lua
	// state for their plugin hooks.
	ulua.L = lua.NewState()
	config.InitRuntimeFiles(false)
	config.InitGlobalSettings()
	config.GlobalSettings["backup"] = false
	config.GlobalSettings["fastdirty"] = true
}

func pluginSource(t *testing.T) string {
	src, err := os.ReadFile("../kanban.lua")
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}

// TestKanbanSpec runs kanban_spec.lua against kanban.lua in gopher-lua, the
// Lua implementation micro embeds.
func TestKanbanSpec(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("PLUGIN_SRC", lua.LString(pluginSource(t)))
	if err := L.DoFile("kanban_spec.lua"); err != nil {
		t.Fatal(err)
	}
}

// TestKanbanGlueSpec runs kanban_glue_spec.lua, which drives the plugin's
// micro-facing callbacks (startup order, bindings, `set kanban.margin`,
// deinit, filetype changes) against stand-in micro modules.
func TestKanbanGlueSpec(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("PLUGIN_SRC", lua.LString(pluginSource(t)))
	if err := L.DoFile("kanban_glue_spec.lua"); err != nil {
		t.Fatal(err)
	}
}

// pluginOn loads the plugin's pure functions (it returns before the micro glue
// because `import` is nil) and exposes a real micro buffer as `buf`.
func pluginOn(t *testing.T, b *buffer.Buffer) *lua.LState {
	L := lua.NewState()
	if err := L.DoString(pluginSource(t)); err != nil {
		t.Fatal(err)
	}
	L.SetGlobal("buf", luar.New(L, b))
	L.SetGlobal("Loc", luar.New(L, func(x, y int) buffer.Loc { return buffer.Loc{X: x, Y: y} }))
	return L
}

const editScript = `
local lines = {}
for y = 0, buf:LinesNum() - 1 do lines[y + 1] = buf:Line(y) end
local board = findBoards(lines)[1]
local edit = %s
applyEdit(buf, lines, edit, Loc)
`

func runEdit(t *testing.T, L *lua.LState, edit string) {
	if err := L.DoString(fmt.Sprintf(editScript, edit)); err != nil {
		t.Fatal(err)
	}
}

// A card move must be a single undo event. micro groups undo by one-second
// clock buckets, so a Remove+Insert pair can be split by Undo (PR #3 review).
func TestMoveIsOneUndoEvent(t *testing.T) {
	const before = "# Demo\n```kanban\n## Todo\n- [ ] a\n  more a\n- [ ] b\n## Done\n- [x] c\n```"
	const after = "# Demo\n```kanban\n## Todo\n- [ ] b\n## Done\n- [x] c\n- [ ] a\n  more a\n```"

	b := buffer.NewBufferFromString(before, "", buffer.BTDefault)
	L := pluginOn(t, b)
	defer L.Close()

	events := b.UndoStack.Len()
	runEdit(t, L, "moveCard(lines, board, 1, 1, 1)")
	if got := string(b.Bytes()); got != after {
		t.Fatalf("move:\n got %q\nwant %q", got, after)
	}
	if n := b.UndoStack.Len() - events; n != 1 {
		t.Fatalf("move recorded %d undo events, want exactly 1", n)
	}

	b.UndoOneEvent() // a single event, regardless of undo's time grouping
	if got := string(b.Bytes()); got != before {
		t.Fatalf("undo:\n got %q\nwant %q", got, before)
	}
	b.Redo()
	if got := string(b.Bytes()); got != after {
		t.Fatalf("redo:\n got %q\nwant %q", got, after)
	}
}

func TestMoveLeftAndToggleUndo(t *testing.T) {
	const before = "```kanban\n## A\n## B\n- [ ] é card\n```"
	const moved = "```kanban\n## A\n- [ ] é card\n## B\n```"
	const toggled = "```kanban\n## A\n- [x] é card\n## B\n```"

	b := buffer.NewBufferFromString(before, "", buffer.BTDefault)
	L := pluginOn(t, b)
	defer L.Close()

	runEdit(t, L, "moveCard(lines, board, 2, 1, -1)")
	if got := string(b.Bytes()); got != moved {
		t.Fatalf("move left:\n got %q\nwant %q", got, moved)
	}
	events := b.UndoStack.Len()
	runEdit(t, L, "toggleCard(lines, board, 1, 1)")
	if got := string(b.Bytes()); got != toggled {
		t.Fatalf("toggle:\n got %q\nwant %q", got, toggled)
	}
	if n := b.UndoStack.Len() - events; n != 1 {
		t.Fatalf("toggle recorded %d undo events, want 1", n)
	}
	b.UndoOneEvent()
	b.UndoOneEvent()
	if got := string(b.Bytes()); got != before {
		t.Fatalf("undo both:\n got %q\nwant %q", got, before)
	}
}

func TestReorderIsOneUndoEvent(t *testing.T) {
	const before = "```kanban\n## A\n- [ ] one\n- [ ] two\n  two more\n```"
	const after = "```kanban\n## A\n- [ ] two\n  two more\n- [ ] one\n```"

	b := buffer.NewBufferFromString(before, "", buffer.BTDefault)
	L := pluginOn(t, b)
	defer L.Close()

	events := b.UndoStack.Len()
	runEdit(t, L, "reorderCard(lines, board, 1, 1, 1)")
	if got := string(b.Bytes()); got != after {
		t.Fatalf("reorder:\n got %q\nwant %q", got, after)
	}
	if n := b.UndoStack.Len() - events; n != 1 {
		t.Fatalf("reorder recorded %d undo events, want 1", n)
	}
	b.UndoOneEvent()
	if got := string(b.Bytes()); got != before {
		t.Fatalf("undo:\n got %q\nwant %q", got, before)
	}
}
