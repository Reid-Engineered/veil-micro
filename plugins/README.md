# veil plugins

Plugins that use the veil decorations API (`docs/API.md`). They live here,
not in `runtime/plugins/`, because everything under `runtime/plugins/` is
compiled into micro and enabled by default for every user.

| Plugin | What it does | Needs |
|---|---|---|
| `concealdemo` | `- [x]` → `✔`, `- [ ]` → `☐` in markdown | conceal rendering (T2, merged) |
| `vlinedemo` | `vlinebox` / `vlinehide` / `vlineclear` commands | virtual rows (T3/T4), not drawn yet |

## Try them

```bash
go build -o ~/.local/bin/micro-dev ./cmd/micro
plugins/install-dev.sh
micro-dev -config-dir ~/.config/micro-dev plugins/concealdemo/demo.md
```

Inside micro-dev, `help concealdemo` and `help vlinedemo` show usage.
