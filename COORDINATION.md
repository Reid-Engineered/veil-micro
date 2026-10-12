# COORDINATION

Read this before starting any work session. Claim before touching files. Never edit a
file another agent has claimed; leave a request in "Requests" instead.

Timestamps: ISO 8601 with offset. Branches: `agent-<name>/<task>`, one git worktree each.
Merge to `main` only via small reviewed merges, rebased on `main` first.

## Baseline
- Upstream: github.com/zyedidia/micro @ `e207f97` (2026-10-07). Requires Go ≥ 1.19
  (go.mod); use a current toolchain. Baseline build/test not yet run (see Log).
- API contract: `docs/API.md` (draft v0.2).
- Shared context for all agents/apps: `PROJECT.md`. Read it first.

## Task board

| ID | Task | Agent | Branch | Files (owned) | Status | Claimed |
|----|------|-------|--------|---------------|--------|---------|
| T0 | Phase 1 recon, plan, API draft, PROJECT.md | recon | — | `PLAN.md`, `docs/API.md`, `COORDINATION.md`, `PROJECT.md` | done | 2026-10-12T12:30-04:00 |
| T1 | Decoration storage + edit semantics + Buffer API methods | A (Claude, chat) | `agent-a/decorations-core` | `internal/buffer/decorations.go`, `decorations_test.go` (new), `internal/buffer/line_array.go`, `internal/config/settings.go`, `runtime/help/options.md` | **done — merged to main (review waived by Marcus)** | 2026-10-12T12:20-04:00 |
| T2 | Conceal rendering + reveal-on-cursor + horizontal mapping | Codex (assigned by Marcus) | `agent-codex/conceal-render` | `internal/display/bufwindow.go` (glyph loop, `getStartInfo`, `LocFromVisual`, minimal prerequisite skeleton), `internal/display/conceal.go`, `internal/display/conceal_test.go` (new) | **done — merged to main as PR #1**; T3 integration pending | 2026-10-12T12:42:27-04:00 |
| T3 | Row model: hidden lines + virtual rows in scroll/diff/mapping | Codex (assigned by Marcus) | `agent-codex/rows` | `internal/display/softwrap.go`, `softwrap_test.go` (new), `bufwindow.go` (`LocFromVisual` delegation only) | done — build/tests pass; awaiting review | 2026-10-12T14:29:58-04:00 |
| T4 | Virtual-row drawing + gutter | B | `agent-b/vlines-render` | `internal/display/bufwindow.go` (row loop / `wrap()` / gutter helpers, `Relocate`) | blocked by T1, T3 | |
| T5 | Cursor up/down + mouse over virtual/hidden rows | B | `agent-b/cursor-move` | `internal/action/actions.go` (`MoveCursorUp/Down`, mouse handlers) | blocked by T3 | |
| T6 | Test plugins (`- [x]`→`✔`, box below a line) | Claude (chat) | `agent-claude/test-plugins` | `plugins/` (new top-level dir: `concealdemo/`, `vlinedemo/`, `install-dev.sh`, `README.md`) | in review (PR) | 2026-10-12T13:41-04:00 |
| T7 | Kanban plugin | C | `agent-c/kanban` | `runtime/plugins/kanban/` | open (stub against API.md) | |
| T8 | Docs: help page + fork README | C | `agent-c/docs` | `runtime/help/plugins.md` (append section), `README.fork.md` | open | |

### bufwindow.go is shared — split rule
`internal/display/bufwindow.go` is touched by both A (T2) and B (T4). To avoid
conflicts: **B lands T4's skeleton first** — it extracts the per-row draw into a helper
and adds a hook `w.concealAt(line, x)` returning `nil`. A then implements that hook in
`internal/display/conceal.go` and only edits the glyph loop inside `displayBuffer`.
Whoever needs a change in the other's region posts a request below.

## Requests
- T3 -> T4/T5 handoff: `getTextRowCount(line)` excludes hidden text;
  `getVirtualRowCount(line)` includes only unsuppressed virtual blocks;
  `getRowCount` is their sum. Text rows precede virtual rows, including the
  existing empty EOL row when wrapped text exactly fills the width. Virtual rows
  are never wrapped and remain visible on a hidden anchor unless revealed.
  `StartLine.Row` can identify a virtual row even with softwrap off; T4 must
  normalize `StartLine` with `Scroll(StartLine, 0)` and consume this row ordering
  when drawing. All-hidden views return `{0,0}` as an empty-view sentinel.
  `LocFromVLoc` maps virtual rows to anchor EOL. T5 must skip them during cursor
  movement to avoid landing back on the same anchor. T4/T5 remain separate tasks.
- T3 (Codex): claim includes removing T2's duplicate non-softwrap conceal
  shortcut in `LocFromVisual`; all horizontal and virtual-row hit testing must
  delegate to the unified `LocFromVLoc` row model. T4 drawing/Relocate and T5
  action ownership remain unchanged.
- 2026-10-12T13:31:52-04:00 T2 (Codex): reclaiming the same T2 files
  for Claude's PR #1 fixes. No expansion into B's files or row-model work.
- 2026-10-12T12:42:27-04:00 T2 (Codex): Marcus authorized preparing the minimal T4
  drawing skeleton prerequisite for T2. No virtual-row implementation or changes
  to B's `softwrap.go`, `Relocate`, gutter or action ownership are claimed.
  The prerequisite is commit `593db04f`: shared `drawRowGutter` plus a nil
  `concealAt` hook, followed by T2's implementation in the next commit. The
  unused hook was subsequently removed per Claude's review; `concealGlyphs`
  is the shared conceal drawing interface instead.
- T3/T4 handoff: use `concealGlyphs(line)` (`nil` means the raw fast path; an
  empty non-nil slice means no displayed characters), `displayGlyph` source
  `col/end` + `visualX/width`, and `decorationSuppressed(line, group)`. Synthetic
  replacement glyphs map to their source start and bypass cursor/selection styling.
  Preserve the same tab/wordwrap rules as `displayBuffer`. T2 supplies non-softwrap
  horizontal hit testing; B still owns wrapped row counts, scrolling and mapping.
  Until T3 consumes these helpers, wrapped navigation uses the raw-text row model.

## API changes
- 2026-10-12 v0.2 (approved by Marcus): rune columns confirmed; `ConcealLines` confirmed;
  per-cell colour for virtual lines: `SetVirtualLines(line, texts, group)` drops the
  `styles` arg, new `StyleVirtualLine(line, row, startCol, endCol, style, group)`.
- 2026-10-12 v0.3 (T1, additive): `AddConceal`/`StyleVirtualLine`/`SetRevealMode` return
  bool; columns defined as character indices (Loc.X units); read accessors
  `LineConceals`, `LineHiddenBy`, `LineVirtualBlocks`, `LineGroups`, `GroupRevealMode`;
  split-at-column-0 and join semantics documented.

## Conflicts / overlaps found
_(none)_

## Log
- 2026-10-12T14:39:36-04:00 T3 (Codex): implemented hidden/virtual row counts,
  zero-row-safe Scroll/Diff, stale-location normalization, and conceal-aware
  wrapped/unwrapped source/visual mapping using T2's shared glyph sequence.
  Removed only the duplicate LocFromVisual shortcut in bufwindow.go; no drawing
  or action changes and no Lua API changes. Nine T3 tests plus 100 randomized
  screen/mapping comparisons pass; existing T2 tests, `make build`, `make test`
  and display race tests pass on Go 1.26.0 WSL. Review required before merge.
  T4 still must draw hidden/virtual rows and consume the new StartLine.Row model;
  T5 still must skip virtual rows for keyboard movement. No user install performed.
- 2026-10-12T14:25-04:00 T6 (Claude, chat): fixed Codex's follow-up P1 (infinite loop when an
  unnamed buffer is open before `init()`): buffers are registered only in `onBufferOpen`, and
  `init()` iterates a snapshot. Verified in a Lua harness (fengari, stubbed micro modules):
  the previous commit hangs on startup-order + unnamed buffer; the fix passes startup,
  disable/re-enable, filetype changes, focus-change rescan and no-restore-after-disable.
- 2026-10-12T14:15-04:00 T6 (Claude, chat): addressed Codex's PR #2 review: `vlinebox` args
  via Go `strings.Join` (P1 crash); concealdemo clears its group on `deinit` and when a
  buffer leaves markdown, guards pending rescans, and debounces per buffer, resolving the
  edited buffer at edit time (P2s); README and PLAN.md §0 now build with `make build`
  (bare `go build` skips `go generate`, so `.md` opens as `unknown`). Lua syntax-checked;
  needs a re-run of Codex's runtime repro.
- 2026-10-12T13:50-04:00 T6 (Claude, chat): demo plugins moved from `runtime/plugins/` to a
  top-level `plugins/` dir, because everything under `runtime/plugins/` is embedded in the
  binary and enabled by default for all users. `plugins/install-dev.sh` installs them into
  `~/.config/micro-dev/plug` only. Lua syntax-checked; not run in micro (no Go build in the
  chat sandbox). `vlinedemo` draws nothing until T3/T4 land. T7 should also live in `plugins/`.
- 2026-10-12T13:32:37-04:00 T2 review follow-up (Codex): addressed Claude's three
  requested fixes in PR #1: removed unused `concealAt`, documented T3/T4
  use of `decorationSuppressed`, restored upstream wrap condition, corrected
  pushed status. Tests now exercise the actual glyph path instead of the removed
  hook. Uncached display tests, `make test`, display race tests and `go build ./...`
  pass. Claude's verdict was conditional approval after these fixes; PR remains
  unmerged. T3 softwrap row-count/scroll/click integration is still pending.
- 2026-10-12T12:54:42-04:00 T2 (Codex): implemented off-by-default conceal glyph rendering,
  theme styles, window-local cursor/selection/group reveal, concealed horizontal
  starts and non-softwrap clicks; no Lua API or T1 storage changes. `make build`,
  `make test`, uncached internal/cmd tests and display race tests pass on WSL.
  Added 12 display tests, including 80 drawing comparisons against substituted
  ordinary text and 40 Unicode/tab width cases. Minimal drawing prerequisite was
  explicitly authorized by Marcus; T3/T4/action files remain under B's ownership.
  Pushed as PR #1; not merged to main or installed as micro-dev.
  Real Windows Terminal behavior and wrapped navigation after T3 remain unverified.
- 2026-10-12 12:35 T1 merged to `main` (fast-forward). Marcus waived the review for T1.
  T2 and T3 are now unblocked.
- 2026-10-12 12:31 T1: `go build ./...` + `go test ./internal/...` green on Marcus's WSL box.
  Needs a review by a different agent, then merge to `main`.
- 2026-10-12 T1 (Claude, chat): implemented on `agent-a/decorations-core`. `buffer.go` did
  NOT need changes (hook lives in `LineArray.insert/remove`), so it's released. Added
  the `conceal` entry to `runtime/help/options.md`. Not compiled: the chat sandbox's
  network policy blocks Go module downloads (403). Must pass `go test ./internal/...`
  on WSL before T1 is marked done.
- 2026-10-12 recon: plan + API draft written. Go module proxy unreachable from the
  recon sandbox, so baseline build/test must be run on the WSL box first (see PLAN.md §0).
