# Phase 1 — Recon & Plan

Baseline: micro `master` @ `e207f97` (2026-10-07). Line numbers below refer to that commit.

## 0. Environment (WSL2)

```bash
# Go: go.mod says `go 1.19`; Ubuntu's apt Go is usually fine, but use a current tarball anyway
curl -LO https://go.dev/dl/go1.24.7.linux-amd64.tar.gz      # or newest 1.2x
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.24.7.linux-amd64.tar.gz
echo 'export PATH=/usr/local/go/bin:$HOME/go/bin:$HOME/.local/bin:$PATH' >> ~/.bashrc

mkdir -p ~/src && cd ~/src
git clone git@github.com:<you>/micro.git && cd micro      # your fork, on the Linux FS
git remote add upstream https://github.com/zyedidia/micro.git

# baseline: must be green before anyone writes feature code
go build -o ~/.local/bin/micro-dev ./cmd/micro
go test ./internal/...

# per-agent worktrees
git worktree add ../micro-a -b agent-a/decorations-core
git worktree add ../micro-b -b agent-b/rows
git worktree add ../micro-c -b agent-c/test-plugins
```

`micro-dev` and your stable `micro` share `~/.config/micro`. To keep the dev build's
plugins and settings separate, run it as `micro-dev -config-dir ~/.config/micro-dev`
(alias it).

**Windows Terminal:** box-drawing (`U+2500–257F`) and `✔ U+2714` render at width 1 in
Cascadia Mono / Cascadia Code (the default) and in most Nerd Fonts. Two traps:
- box-drawing chars are East-Asian *ambiguous* width. micro uses go-runewidth, which
  treats them as width 1 unless `RUNEWIDTH_EASTASIAN=1` or a CJK locale is set. Keep
  `LANG=en_US.UTF-8`.
- don't append `U+FE0F` (emoji presentation) to `✔`; Windows Terminal would then draw it
  2 cells wide while micro counts 1, shifting the rest of the row.

## 1. How micro renders (the parts that matter)

```
BufPane (internal/action)           ── keys, mouse, plugin callbacks
   │  MoveCursorUp/Down, mouse → LocFromVisual
   ▼
BufWindow (internal/display)
   ├─ bufwindow.go: displayBuffer()  ── one loop over screen rows; draws gutter, glyphs
   ├─ bufwindow.go: Relocate()       ── keeps cursor in view (uses Scroll/SLocFromLoc)
   └─ softwrap.go: SoftWrap iface    ── ALL screen↔buffer mapping:
          Scroll, Diff, SLocFromLoc, VLocFromLoc, LocFromVLoc, getRowCount
   ▼
Buffer → SharedBuffer → LineArray → []Line   (internal/buffer)
```

Key facts found:

1. **Every coordinate conversion goes through `softwrap.go`.** Callers
   (`Relocate`, `LocFromVisual` for mouse, `actions.go` cursor/page moves, `bufpane.go`
   `GotoLoc`-style centering) only use `Scroll / Diff / SLocFromLoc / VLocFromLoc /
   LocFromVLoc`. This is the single most important finding: if those five functions
   are decoration-aware, cursor, scrolling, mouse and page-up/down are correct "for
   free". Search highlight and selection are drawn in buffer coordinates inside
   `displayBuffer`, so they're unaffected as long as the glyph loop keeps `bloc` right.
2. The model is already **"a buffer line occupies N screen rows"** (`SLoc{Line,Row}`,
   `getRowCount(line)`) — that's how soft wrap works. Virtual lines and hidden lines
   fit this model exactly: `rows(line) = (hidden ? 0 : wrappedRows) + len(virtual)`.
   That's why I'm confident virtual lines are feasible without restructuring.
3. But with softwrap **off**, `Scroll`/`Diff`/`SLocFromLoc` take a shortcut that
   assumes exactly 1 row per line (`softwrap.go:292–318`). When a buffer has
   decorations we must take the row-counting path even if softwrap is off.
4. **`SharedBuffer.insert/remove` (`buffer.go:126–140`) is the one choke point for all
   edits** — typing, undo/redo, replace-all, multi-cursor, `ApplyDiff`. (By contrast,
   `EventHandler.DoTextEvent` only shifts cursors for single-delta events, so it's the
   wrong hook.) `MarkModified(start,end)` there already tells us which lines changed.
5. **`buffer.Line` already carries per-line side data** (`state`, `match`, and a
   `search map[*Buffer]*searchState`). Putting decorations on `Line` means line
   insertion/deletion moves them automatically, and there's upstream precedent for it.
6. **Lua bindings are automatic.** `cmd/micro/initlua.go` hands Go objects to Lua via
   gopher-luar; any exported `*Buffer` method is callable as `buf:Foo()`. No binding
   file to write — the "Lua binding" work is just choosing Lua-friendly Go signatures.
7. Plugin hooks available: `onBeforeTextEvent`, `on<Action>`/`pre<Action>` for every
   action (`bufpane.go:560–585`), `onRune`, `onBufferOpen`, `onBufferOptionChanged`, and
   `micro.After(duration, fn)` for debouncing. Enough for the kanban plugin; there is no
   generic "cursor moved" hook, which is one reason reveal-on-cursor belongs in core.
8. Splits of the same file share one `SharedBuffer` but each has its own `*Buffer` and
   cursors, so decorations are shared and reveal is per-window — the behaviour we want.

## 2. Design decisions (and why)

**D1 — Decorations stored on `buffer.Line`, conceals invalidated on edit.**
Shifting column ranges correctly through arbitrary edits is a whole sub-project
(Neovim's extmarks). Instead: line moves are free (data rides on the `Line`), and any
edit touching a line drops its conceals; the plugin recomputes (debounced). Because of
reveal-on-cursor, the line you're editing is never concealed anyway, so the user never
sees stale conceals.

**D2 — Reveal-on-cursor is a core rule, per group.**
Conceals hide text the cursor might be on; letting the core skip decorations on cursor
lines means the cursor's screen column is always computed from raw text, which removes
the hardest mapping cases. Doing it in core (not the plugin) avoids a one-frame flicker
and the missing "cursor moved" hook. A per-group `"group"` mode lets a kanban block
reveal as a unit.

**D3 — Added `ConcealLines` (zero-row lines).** Not in the brief, but required:
concealing every character of the kanban's raw lines would still leave a blank row per
line under the drawn board. Hidden lines are just `rows = 0` in the row model.

**D4 — Columns are rune indices, styles are colorscheme names.** Matches
`buffer.Loc` and existing plugin APIs; no tcell types cross into Lua; theme-able.

**D5 — Virtual rows map to their anchor line.** `LocFromVLoc` on a virtual row returns
the end of the anchor line, so a click on the board puts the cursor on the anchor
(which then reveals the block). Cursor movement skips virtual rows explicitly (R2).

## 3. Exact change list

| File | Function(s) | Change | Owner |
|---|---|---|---|
| `internal/buffer/decorations.go` (new) | types `Conceal`, `VirtualLine`; `AddConceal`, `ConcealLines`, `SetVirtualLines`, `SetRevealMode`, `ClearDecorations`, `HasDecorations`; read helpers for display | All storage + API | A |
| `internal/buffer/line_array.go` | `Line` struct (+`deco *lineDeco`), `split`, `joinLines` | Field; keep deco on first half on split; merge on join | A |
| `internal/buffer/buffer.go` | `SharedBuffer.insert`, `.remove` | Drop conceals on touched lines; maintain group→count index | A |
| `internal/config/settings.go` | `defaultCommonSettings` | `"conceal": false` | A |
| `internal/display/softwrap.go` | `getRowCount`, `getVLocFromLoc`, `getLocFromVLoc`, `Scroll`, `Diff`, `SLocFromLoc`, `VLocFromLoc`, `LocFromVLoc`, `scrollDown/Up` | Row model incl. hidden (0 rows) and virtual rows; take the row path when `HasDecorations()`; handle `rc == 0` in scroll loops | B |
| `internal/display/bufwindow.go` | `displayBuffer` (row loop, `wrap()`), new `drawVirtualRow`, `drawLineNum`/`drawDiffGutter` callers | Skip hidden lines, emit virtual rows with blank gutter; compute per-frame reveal set | B |
| `internal/display/bufwindow.go` | `displayBuffer` glyph loop, `getStartInfo` | Skip concealed runes, emit replacement glyphs (no cursor/selection on them); conceal-aware horizontal start | A |
| `internal/display/conceal.go` (new) | `concealAt`, reveal-set computation | Keep logic out of the 900-line file | A |
| `internal/display/bufwindow.go` | `Relocate` | Uses Scroll/SLocFromLoc → mostly automatic; verify bEnd with hidden tail lines | B |
| `internal/action/actions.go` | `MoveCursorUp`, `MoveCursorDown` (≈ lines 217–252), mouse press/drag | Skip virtual rows; take the VLoc path when decorations exist | B |
| `runtime/plugins/*` | demo + kanban plugins | Lua | C |
| `runtime/help/plugins.md` | append | Document API | C |

Conceal width also affects soft-wrap row counts (`getVLocFromLoc` walks runes), so the
conceal width function is shared by A and B: A provides
`(*Buffer).DisplayRunes(line, reveal)`-style helper in T1, B calls it in T3.

## 4. Risks

- **R1 Two rendering loops must agree.** `getVLocFromLoc` (softwrap.go) and the glyph
  loop in `displayBuffer` each re-implement width/wrap. Conceals must be applied
  identically in both or cursors drift. Mitigation: one shared iterator helper, and a
  test that compares them on random lines.
- **R2 Cursor stuck at virtual rows.** With the VLoc path, moving down from an anchor
  line lands on a virtual row that maps back to the anchor → cursor doesn't move. Must
  loop past virtual rows. Non-softwrap `Cursor.DownN` is fine.
- **R3 Zero-row lines** break `scrollDown/Up` assumptions (`Row` always valid,
  `rc ≥ 1`). Also `StartLine` must never sit on a hidden line. Needs careful tests.
- **R4 Layout jump on reveal.** Entering a kanban block collapses the board and shows
  raw lines; `Relocate` then scrolls. Acceptable for MVP (Obsidian does the same) but
  will feel jumpy on large boards.
- **R5 Horizontal scroll** (softwrap off, `StartCol > 0`): `getStartInfo` must count
  concealed widths for non-cursor lines. Easy to get subtly wrong.
- **R6 Performance.** Reveal computation per frame and row-path scrolling are O(lines
  scanned). Gate everything on `conceal && HasDecorations()` so normal buffers take the
  existing fast path unchanged.
- **R7 Ambiguous-width glyphs** (see §0).
- **R8 Recon sandbox couldn't download Go modules** (proxy blocked), so the baseline
  build/test is unverified — do it first on WSL.

## 5. Upstreaming outlook

Things that help: off by default, zero cost when unused, generic API, no kanban logic in
Go, new code mostly in two new files. Things that could sink it:
- Maintainers are cautious about large display changes; `displayBuffer` is shared by
  every user. **Open an upstream issue/discussion with this API before Phase 2** and
  aim for two PRs (conceal first, virtual lines second) rather than one.
- `ConcealLines` + virtual rows effectively add folding-like behaviour; reviewers may
  ask for real folding instead. Worth framing it as the primitive folding could build on.
- Drop-on-edit conceal semantics (D1) are pragmatic but may be seen as surprising.
- Any change to `SoftWrap` interface signatures would ripple; the plan avoids that.
I didn't find existing upstream work on conceal/virtual text in a quick search, but
check the issue tracker yourself before the first PR.

## 6. Proposed split

As in `COORDINATION.md`: A = storage, edit semantics, conceal rendering; B = row model,
virtual rows, cursor/mouse movement; C = demo plugins, kanban plugin, docs, against
`docs/API.md`. Order: **T1 first (A, small, unblocks everyone)** → T3/T2 in parallel →
T4/T5 → plugins switch from stubs to real API. The one shared file (`bufwindow.go`) has
an explicit hand-off rule in `COORDINATION.md`.

## 7. Decisions (approved 2026-10-12)

1. Rune columns instead of byte ranges: **yes**.
2. `ConcealLines` added to the API: **yes**.
3. Virtual rows get **per-cell colour** in v1 → `StyleVirtualLine` (see `docs/API.md` v0.2).
   Impact: B's virtual-row drawer (T4) takes a span list per row instead of one style.
