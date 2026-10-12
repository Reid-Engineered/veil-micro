# veil plugins

Plugins that use the veil decorations API (`docs/API.md`). They live here,
not in `runtime/plugins/`, because everything under `runtime/plugins/` is
compiled into micro and enabled by default for every user.

| Plugin | What it does | Needs |
|---|---|---|
| `concealdemo` | `- [x]` → `✔`, `- [ ]` → `☐` in markdown | conceal rendering (T2, merged) |
| `vlinedemo` | `vlinebox` / `vlinehide` / `vlineclear` commands | virtual rows (T3/T4), not drawn yet |
| `kanban` | Obsidian-style boards drawn as columns; Alt-l/Alt-h move cards, Alt-t toggles | virtual rows + hidden lines (T3/T4) to draw; edits work now |

## Try them

```bash
make build && install -m 755 micro ~/.local/bin/micro-dev
plugins/install-dev.sh
micro-dev -config-dir ~/.config/micro-dev plugins/concealdemo/demo.md
```

Use `make build`, not a bare `go build ./cmd/micro`. `make build` first runs
`go generate ./runtime`, which creates the syntax header files (`*.hdr`, not
checked in). Without them micro can't detect filetypes, `.md` files open as
`unknown`, and concealdemo never switches on.

Inside micro-dev, `help concealdemo` and `help vlinedemo` show usage.

## Tests

Plugin logic that doesn't need the editor lives in pure Lua functions and is
tested by `*_spec.lua` files under each plugin's `spec/` folder, run through
gopher-lua (the Lua micro embeds). kanban's Go test also drives a real micro
buffer to check that a card move is exactly one undo event:

```bash
go test ./plugins/...
```

Specs live in `spec/` because micro loads every top-level `.lua` file in a
plugin folder, and because the repo's `.gitignore` ignores any `test/` directory.

