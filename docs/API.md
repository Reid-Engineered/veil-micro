# Decorations Lua API (v0.3, storage implemented in T1)

Status: **draft, Phase 1**. Any change to signatures or semantics must be announced in
`COORDINATION.md` (section "API changes") *before* it is merged, because the plugin
side (Agent C) codes against this file.

Baseline: micro `master` @ `e207f97` (2026-10-07), module `github.com/micro-editor/micro/v2`.

## How plugins reach it

No binding code is needed. micro exposes Go objects to Lua through gopher-luar
reflection, so every **exported method on `*buffer.Buffer`** is callable from Lua as
`bp.Buf:Method(...)`. The API below is therefore just a set of exported Go methods.
Lua tables passed as `[]string` are converted automatically by luar.

## Conventions

- Decisions D1–D5 and questions Q1–Q3 in PLAN.md are **approved** (2026-10-12).
- Lines and columns are **0-based**. Columns are **character indices**, the same unit
  as `buffer.Loc.X` and existing APIs like `buf:Line(n)` (a combining mark counts with
  its base character, so this is slightly more than "runes"). (The project brief said "byte
  range"; micro's whole coordinate system is rune-based, so byte offsets would force
  every caller to convert. Changed deliberately.)
- Ranges are half-open: `[startCol, endCol)`.
- `group` is a plugin-chosen string (e.g. `"kanban:12"`, `"checkbox"`). Groups are the
  unit of clearing and of reveal-on-cursor.
- `style` is a **colorscheme group name** (e.g. `"special"`, `"kanban.done"`), resolved
  through `config.Colorscheme` at draw time with fallback to the default style. This
  keeps tcell types out of the Lua API and makes colors theme-able.
- Nothing renders unless the buffer option `conceal` is `true` (default `false`).
- Decorations live on the shared line storage, so two splits of the same file see the
  same decorations; reveal-on-cursor is computed **per window** from that window's
  cursors.

## Methods

### `buf:AddConceal(line, startCol, endCol, replacement, style, group) -> bool`
Hide `[startCol, endCol)` on `line` and draw `replacement` (may be `""`) in its place,
using `style`. Columns are clamped to the line. Returns `false` (and adds nothing) if
the line doesn't exist, the clamped range is empty, or it overlaps an existing conceal
on that line (first added wins). Newlines in `replacement` become spaces.

### `buf:ConcealLines(startLine, endLine, group)`
Hide whole lines `[startLine, endLine]` (inclusive): they take **zero screen rows**.
Virtual lines anchored to a hidden line are still drawn. (Not in the original brief —
needed so a kanban block's raw markdown can disappear instead of leaving blank rows.)

### `buf:SetVirtualLines(line, texts, group)`
Attach display-only rows **below** `line`. `texts` is a list of strings, drawn in the
default style until styled with `StyleVirtualLine`. Replaces any virtual lines (and
their styles) previously set on `line` by the same `group`. Virtual rows have a blank
gutter, can't hold the cursor, are not soft-wrapped (truncated at window width), and
scroll horizontally with `StartCol` when softwrap is off.

### `buf:StyleVirtualLine(line, row, startCol, endCol, style, group) -> bool`
Per-cell colour. Applies colorscheme group `style` to rune columns `[startCol, endCol)`
of virtual row `row` (0-based) attached to `line` by `group`. Later calls override
earlier ones where they overlap. `endCol = -1` means "to end of row". Returns `false`
if that row doesn't exist or the span is empty. Replacing the rows with
`SetVirtualLines` resets their styles.
Why separate calls instead of nested `{ {text, style}, ... }` tables: every argument is
a plain int/string/`[]string`, which gopher-luar converts reliably; nested Lua tables
to Go structs are fragile.

Example (one kanban row, coloured title and tag):
```lua
buf:SetVirtualLines(12, {"│ Fix gutter  #bug │"}, "kanban:12")
buf:StyleVirtualLine(12, 0, 0, -1, "kanban.border", "kanban:12")
buf:StyleVirtualLine(12, 0, 2, 12, "kanban.card",   "kanban:12")
buf:StyleVirtualLine(12, 0, 13, 17, "kanban.tag",   "kanban:12")
```

### `buf:SetRevealMode(group, mode) -> bool`
`mode` is `"line"` (default) or `"group"`.
- `"line"`: a decoration is suppressed on any line that holds a cursor or is touched by
  a selection in this window.
- `"group"`: if any cursor/selection in the window touches *any* line carrying a
  decoration of this group (including a virtual-line anchor), the **whole group** is
  suppressed. The kanban plugin uses one group per block in this mode.

Returns `false` for an unknown mode. Modes survive `ClearDecorations`.

### `buf:ClearDecorations(group)`
Remove every decoration in `group`. `""` clears all groups.

### `buf:HasDecorations() -> bool`
Cheap check, also used by the core to choose the fast (uniform-rows) display path. May
return `true` after edits removed the last decoration; never `false` while any exist.

### Read accessors (used by the display code; also callable from Lua)
- `buf:LineConceals(line)` → list of `Conceal{Start, End, Replacement, Style, Group}`,
  sorted by `Start`, non-overlapping
- `buf:LineHiddenBy(line)` → groups hiding the line
- `buf:LineVirtualBlocks(line)` → list of `VirtualBlock{Group, Rows}`; each row is
  `VirtualRow{Text, Spans}`, span `VirtualSpan{Start, End, Style}`
- `buf:LineGroups(line)` → every group with any decoration on the line (reveal uses this)
- `buf:GroupRevealMode(group)` → `RevealLine` (0) or `RevealGroup` (1)
Treat returned slices as read-only.

## Behaviour under edits

- Decorations are stored on the per-line objects (`buffer.Line`), so inserting or
  deleting other lines moves them for free.
- Any edit that touches a line **drops that line's conceals** (column offsets are no
  longer trustworthy). Virtual lines and line-hiding stay attached to the line object;
  if the line is deleted they go with it.
- Splitting a line keeps its decorations on the first half, except a split at column 0
  (Enter at the start of the line), which moves them down with the text.
- Joining lines (deleting a line break) keeps the first line's decorations and drops
  the joined-away line's.
- Undo/redo are ordinary edits and follow the same rules.
- Plugins are expected to recompute on `onBeforeTextEvent`/action callbacks
  (debounced via `micro.After`). The core never guesses.

## Not in v0.1

Inline virtual text (inlay hints), virtual lines above a line, mouse hit-testing of virtual rows (clicks map to the anchor line).
