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
| T1 | Decoration storage + edit semantics + Buffer API methods | A (Claude, chat) | `agent-a/decorations-core` | `internal/buffer/decorations.go`, `decorations_test.go` (new), `internal/buffer/line_array.go`, `internal/config/settings.go`, `runtime/help/options.md` | **tests pass on WSL; awaiting review before merge** | 2026-10-12T12:20-04:00 |
| T2 | Conceal rendering + reveal-on-cursor + horizontal mapping | A | `agent-a/conceal-render` | `internal/display/bufwindow.go` (`displayBuffer` glyph loop, `getStartInfo`), `internal/display/conceal.go` (new) | blocked by T1 | |
| T3 | Row model: hidden lines + virtual rows in scroll/diff/mapping | B | `agent-b/rows` | `internal/display/softwrap.go` | blocked by T1 | |
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
_(none)_

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
- 2026-10-12 12:31 T1: `go build ./...` + `go test ./internal/...` green on Marcus's WSL box.
  Needs a review by a different agent, then merge to `main`.
- 2026-10-12 T1 (Claude, chat): implemented on `agent-a/decorations-core`. `buffer.go` did
  NOT need changes (hook lives in `LineArray.insert/remove`), so it's released. Added
  the `conceal` entry to `runtime/help/options.md`. Not compiled: the chat sandbox's
  network policy blocks Go module downloads (403). Must pass `go test ./internal/...`
  on WSL before T1 is marked done.
- 2026-10-12 recon: plan + API draft written. Go module proxy unreachable from the
  recon sandbox, so baseline build/test must be run on the WSL box first (see PLAN.md §0).
