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
| T2 | Conceal rendering + reveal-on-cursor + horizontal mapping | Codex (assigned by Marcus) | `agent-codex/conceal-render` | `internal/display/bufwindow.go` (glyph loop, `getStartInfo`, `LocFromVisual`, minimal prerequisite skeleton), `internal/display/conceal.go`, `internal/display/conceal_test.go` (new) | done — Claude review fixes tested; merge and T3 integration pending | 2026-10-12T12:42:27-04:00 |
| T3 | Row model: hidden lines + virtual rows in scroll/diff/mapping | B | `agent-b/rows` | `internal/display/softwrap.go` | open | |
| T4 | Virtual-row drawing + gutter | B | `agent-b/vlines-render` | `internal/display/bufwindow.go` (row loop / `wrap()` / gutter helpers, `Relocate`) | blocked by T1, T3 | |
| T5 | Cursor up/down + mouse over virtual/hidden rows | B | `agent-b/cursor-move` | `internal/action/actions.go` (`MoveCursorUp/Down`, mouse handlers) | blocked by T3 | |
| T6 | Test plugins (`- [x]`→`✔`, box below a line) | C | `agent-c/test-plugins` | `runtime/plugins/concealdemo/`, `runtime/plugins/vlinedemo/` | open (stub against API.md) | |
| T7 | Kanban plugin | C | `agent-c/kanban` | `runtime/plugins/kanban/` | open (stub against API.md) | |
| T8 | Docs: help page + fork README | C | `agent-c/docs` | `runtime/help/plugins.md` (append section), `README.fork.md` | open | |

### bufwindow.go is shared — split rule
`internal/display/bufwindow.go` is touched by both A (T2) and B (T4). To avoid
conflicts: **B lands T4's skeleton first** — it extracts the per-row draw into a helper
and adds a hook `w.concealAt(line, x)` returning `nil`. A then implements that hook in
`internal/display/conceal.go` and only edits the glyph loop inside `displayBuffer`.
Whoever needs a change in the other's region posts a request below.

## Requests
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
