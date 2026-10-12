-- Tests for kanban.lua's pure logic (parsing, layout, edits).
-- Lives in spec/ because micro loads every top-level .lua file in a plugin dir
-- (and the repo .gitignore ignores directories named test/).
-- Runs under gopher-lua via kanban_test.go (`go test ./plugins/kanban/spec/`), and
-- under any Lua 5.1+ with PLUGIN_SRC set to the plugin source.
-- The plugin returns early when `import` is nil, so no micro stubs are needed.

local load_ = loadstring or load
assert(load_(PLUGIN_SRC))()

local failures = 0
local function check(name, cond, detail)
    if not cond then
        failures = failures + 1
        print("FAIL " .. name .. (detail and (": " .. tostring(detail)) or ""))
    end
end

local function split(text)
    local t = {}
    for line in string.gmatch(text .. "\n", "(.-)\n") do
        table.insert(t, line)
    end
    return t
end

-- apply a {lo, hi, lines} edit to a line array (what the micro glue does)
local function applyEdit(lines, e)
    local out = {}
    for y = 0, e.lo - 1 do table.insert(out, lines[y + 1]) end
    for _, l in ipairs(e.lines) do table.insert(out, l) end
    for y = e.hi + 1, #lines - 1 do table.insert(out, lines[y + 1]) end
    return out
end

local function ulen(s)
    local n = 0
    for i = 1, #s do
        local b = string.byte(s, i)
        if b < 128 or b >= 192 then n = n + 1 end
    end
    return n
end

------------------------------------------------------------------ parsing
local obsidian = split([[
---

kanban-plugin: basic

---

## Todo

- [ ] write parser
- [ ] lay out
  columns side by side

## Doing

- [ ] T3 row model


## Done

- [x] T1 storage
- plain card



%% kanban:settings
```
{"kanban-plugin":"basic"}
```
%%]])

local boards = findBoards(obsidian)
check("obsidian: one board", #boards == 1, #boards)
local b = boards[1]
check("obsidian: kind", b.kind == "file")
check("obsidian: three columns", #b.columns == 3, #b.columns)
check("obsidian: titles", b.columns[1].title == "Todo" and b.columns[3].title == "Done")
check("obsidian: first line is first heading", b.first == 6, b.first)
check("obsidian: last line is last card, settings excluded", b.last == 20, b.last)
local todo = b.columns[1].cards
check("obsidian: todo cards", #todo == 2, #todo)
check("obsidian: continuation joined", todo[2].text == "lay out columns side by side", todo[2].text)
check("obsidian: continuation range", todo[2].line == 9 and todo[2].endLine == 10)
check("obsidian: done flag", b.columns[3].cards[1].done == true and todo[1].done == false)
check("obsidian: plain card", b.columns[3].cards[2].done == nil and b.columns[3].cards[2].text == "plain card")

local fenced = split([[
# Notes

```kanban
## A
- [ ] one
## B
```

```kanban
## C
- [x] two
```
text after
```kanban
## never closed]])
boards = findBoards(fenced)
check("fenced: two closed boards", #boards == 2, #boards)
check("fenced: hides fences", boards[1].first == 2 and boards[1].last == 6, boards[1].first .. "," .. boards[1].last)
check("fenced: second board", boards[2].first == 8 and boards[2].columns[1].cards[1].done == true)
check("fenced: empty column kept", #boards[1].columns[2].cards == 0)

check("not a board without front matter key",
    #findBoards(split("---\ntitle: x\n---\n## A\n- [ ] a")) == 0)
check("### is not a column",
    #findBoards(split("```kanban\n### sub\n- [ ] a\n```")) == 0)

-- a ```kanban example inside another code fence is literal text, not a board
local nested = split([[
Example:

````markdown
```kanban
## A
- [ ] literal
```
````

```kanban
## Live
- [ ] real
```]])
boards = findBoards(nested)
check("nested: only the live board", #boards == 1, #boards)
check("nested: live board found", boards[1] and boards[1].columns[1].title == "Live")

local tilde = split("~~~\n```kanban\n## X\n- [ ] y\n```\n~~~")
check("tilde fence hides a kanban example", #findBoards(tilde) == 0)

local longclose = split("```kanban\n## A\n- [ ] a\n`````")
check("closing run may be longer", #findBoards(longclose) == 1)

local unclosed = split("```python\nprint()\n```kanban\n## A\n```")
check("unterminated fence swallows the rest", #findBoards(unclosed) == 0)

check("fenceOpen info word", select(2, fenceOpen("```kanban extra")) == "kanban")
check("backtick info can't contain backticks", fenceOpen("``` a`b") == nil)

------------------------------------------------------------------ cardAt
boards = findBoards(obsidian)
local bb, ci, ki = cardAt(boards, 10)
check("cardAt continuation line", bb ~= nil and ci == 1 and ki == 2)
bb, ci, ki = cardAt(boards, 6)
check("cardAt heading: board but no card", bb ~= nil and ci == nil)
bb = cardAt(boards, 0)
check("cardAt outside", bb == nil)

------------------------------------------------------------------ layout
local rows, spans = layout(boards[1], 80)
local w = ulen(rows[1])
check("layout: fits 80 columns", w <= 80, w)
local same = true
for _, r in ipairs(rows) do
    if ulen(r) ~= w then same = false end
end
check("layout: all rows same width", same)
check("layout: box corners", string.find(rows[1], "^┌") ~= nil and string.find(rows[#rows], "┘$") ~= nil)
check("layout: header has counts", string.find(rows[2], "Todo %(2%)") ~= nil, rows[2])
check("layout: card glyphs", string.find(rows[4], "☐ write parser") ~= nil and string.find(rows[4], "✔ T1 storage") ~= nil, rows[4])
check("layout: plain card bullet", string.find(rows[5], "• plain card") ~= nil, rows[5])
local borderSpans = 0
for _, s in ipairs(spans) do
    if s[3] == -1 then borderSpans = borderSpans + 1 end
    check("layout: span row in range", s[1] >= 0 and s[1] < #rows)
end
check("layout: every row styled as border first", borderSpans == #rows, borderSpans)

-- narrow window: columns clamp to the minimum width instead of vanishing
rows = layout(boards[1], 20)
check("layout: narrow still draws", #rows > 4 and ulen(rows[1]) > 20)

-- long words wrap inside the column
local longb = findBoards(split("```kanban\n## A\n- [ ] " .. string.rep("x", 60) .. "\n```"))[1]
rows = layout(longb, 40)
for i = 4, #rows - 1 do
    check("layout: wrapped row width", ulen(rows[i]) == ulen(rows[1]), rows[i])
end

-- glyph padding: one extra space after the mark, continuation aligned to the text
local padb = findBoards(split("```kanban\n## A\n- [ ] first card wraps onto a second line here\n- [x] done\n```"))[1]
local prow = layout(padb, 40, 1)
check("pad: todo mark followed by two spaces", string.find(prow[4], "☐  first", 1, true) ~= nil, prow[4])
local textcol = string.find(prow[4], "first", 1, true)
local contcol = string.find(prow[5], "%S", string.find(prow[5], "│", 1, true) + 3)
check("pad: continuation aligned with card text", ulen(string.sub(prow[5], 1, contcol - 1)) == ulen(string.sub(prow[4], 1, textcol - 1)),
    prow[4] .. " / " .. prow[5])
local doneRow
for _, r in ipairs(prow) do if string.find(r, "✔", 1, true) then doneRow = r end end
check("pad: done mark padded", doneRow ~= nil and string.find(doneRow, "✔  done", 1, true) ~= nil, doneRow)
local same2 = true
for _, r in ipairs(prow) do if ulen(r) ~= ulen(prow[1]) then same2 = false end end
check("pad: rows still equal width", same2)
check("pad 0 is the old layout", string.find(layout(padb, 40, 0)[4], "☐ first", 1, true) ~= nil)

-- excessive, negative and fractional padding never drops card text (narrow column)
local narrow = findBoards(split("```kanban\n## A\n- [ ] abcdefghij\n```"))[1]
for _, p in ipairs({ 0, 1, 6, 8, 100, -2, 1.7 }) do
    local rws = layout(narrow, 12, p)
    local text = ""
    for i = 4, #rws - 1 do
        -- row is "│ " .. content .. " │": take the content, drop the mark and spaces
        local cell = string.match(rws[i], "^│ (.*) │$") or ""
        text = text .. string.gsub(string.gsub(cell, "☐", ""), "%s", "")
    end
    check("pad " .. p .. ": all card text kept", text == "abcdefghij", text)
    local w = ulen(rws[1])
    for i = 2, #rws do
        check("pad " .. p .. ": row width", ulen(rws[i]) == w, rws[i])
    end
end

------------------------------------------------------------------ moveCard
boards = findBoards(obsidian)
local e = moveCard(obsidian, boards[1], 1, 1, 1) -- "write parser" -> Doing
local moved = applyEdit(obsidian, e)
check("move right: line count unchanged", #moved == #obsidian)
local nb = findBoards(moved)[1]
check("move right: removed from Todo", #nb.columns[1].cards == 1 and nb.columns[1].cards[1].text == "lay out columns side by side")
check("move right: appended to Doing", #nb.columns[2].cards == 2 and nb.columns[2].cards[2].text == "write parser")
check("move right: cursor on moved card", moved[e.cursor + 1] == "- [ ] write parser", moved[e.cursor + 1])

e = moveCard(moved, nb, 2, 2, -1) -- and back to Todo
local back = applyEdit(moved, e)
nb = findBoards(back)[1]
check("move left: back in Todo (at end)", #nb.columns[1].cards == 2 and nb.columns[1].cards[2].text == "write parser")
check("move left: cursor", back[e.cursor + 1] == "- [ ] write parser", back[e.cursor + 1])

-- multi-line card moves as a block
e = moveCard(obsidian, boards[1], 1, 2, 2) -- Todo card 2 (two lines) -> Done
moved = applyEdit(obsidian, e)
nb = findBoards(moved)[1]
local last = nb.columns[3].cards[#nb.columns[3].cards]
check("move block: continuation kept", last.text == "lay out columns side by side" and last.endLine == last.line + 1, last.text)

-- into an empty column (fenced board)
local f2 = split("```kanban\n## A\n- [ ] one\n## B\n```")
local fb = findBoards(f2)[1]
e = moveCard(f2, fb, 1, 1, 1)
moved = applyEdit(f2, e)
fb = findBoards(moved)[1]
check("move into empty column", #fb.columns[1].cards == 0 and #fb.columns[2].cards == 1, table.concat(moved, "|"))
check("move into empty: fences intact", moved[1] == "```kanban" and moved[#moved] == "```", table.concat(moved, "|"))

-- edges
local _, err = moveCard(f2, findBoards(f2)[1], 1, 1, -1)
check("move left from first column refused", err == "already in the first column", err)
_, err = moveCard(moved, fb, 2, 1, 1)
check("move right from last column refused", err == "already in the last column", err)

-- target column heading is the last line of the file
local f3 = split("```kanban\n## A\n- [ ] one\n- [ ] two\n## B\n```")
local f3b = findBoards(f3)[1]
e = moveCard(f3, f3b, 1, 2, 1)
moved = applyEdit(f3, e)
check("move last card into empty last column", moved[5] == "- [ ] two" and moved[6] == "```", table.concat(moved, "|"))

local f4 = split("```kanban\n## A\n- [ ] one\n## B")
-- (unterminated: no board) -> nothing to move
check("unterminated fence is not a board", #findBoards(f4) == 0)

------------------------------------------------------------------ editLines
boards = findBoards(obsidian)
e = moveCard(obsidian, boards[1], 1, 1, 1)
local ds = editLines(obsidian, e)
check("editLines: only changed lines", #ds > 0 and #ds <= e.hi - e.lo + 1, #ds)
local after = {}
for i, l in ipairs(obsidian) do after[i] = l end
for _, d in ipairs(ds) do
    check("editLines: old text matches", obsidian[d.y + 1] == d.old)
    check("editLines: single-line text", string.find(d.new, "\n") == nil)
    after[d.y + 1] = d.new
end
local viaEdit = {}
do
    local tmp = {}
    for y = 0, e.lo - 1 do table.insert(tmp, obsidian[y + 1]) end
    for _, l in ipairs(e.lines) do table.insert(tmp, l) end
    for y = e.hi + 1, #obsidian - 1 do table.insert(tmp, obsidian[y + 1]) end
    viaEdit = tmp
end
check("editLines reproduces the edit", table.concat(after, "\n") == table.concat(viaEdit, "\n"))

------------------------------------------------------------------ reorderCard
local col = split("```kanban\n## A\n- [ ] one\n- [ ] two\n  two more\n\n- [ ] three\n## B\n```")
local cb = findBoards(col)[1]
check("reorder fixture", #cb.columns[1].cards == 3)

e = reorderCard(col, cb, 1, 1, 1) -- "one" down past "two" (multi-line)
moved = applyEdit(col, e)
check("reorder down: order", moved[3] == "- [ ] two" and moved[4] == "  two more" and moved[5] == "- [ ] one",
    table.concat(moved, "|"))
check("reorder down: cursor on moved card", moved[e.cursor + 1] == "- [ ] one", e.cursor)
check("reorder down: line count", #moved == #col)

local cb2 = findBoards(moved)[1]
e = reorderCard(moved, cb2, 1, 2, -1) -- and back up
local back2 = applyEdit(moved, e)
check("reorder up restores", table.concat(back2, "\n") == table.concat(col, "\n"), table.concat(back2, "|"))
check("reorder up: cursor", back2[e.cursor + 1] == "- [ ] one", e.cursor)

e = reorderCard(col, cb, 1, 2, 1) -- "two" down past "three", blank line stays between
moved = applyEdit(col, e)
check("reorder keeps blank between cards",
    moved[3] == "- [ ] one" and moved[4] == "- [ ] three" and moved[5] == "" and moved[6] == "- [ ] two"
    and moved[7] == "  two more", table.concat(moved, "|"))
check("reorder blank: cursor", moved[e.cursor + 1] == "- [ ] two", e.cursor)

local _, rerr = reorderCard(col, cb, 1, 1, -1)
check("reorder first card up refused", rerr == "already the first card in this column", rerr)
_, rerr = reorderCard(col, cb, 1, 3, 1)
check("reorder last card down refused", rerr == "already the last card in this column", rerr)

------------------------------------------------------------------ toggle
boards = findBoards(obsidian)
e = toggleCard(obsidian, boards[1], 1, 1)
check("toggle todo -> done", e.lines[1] == "- [x] write parser", e.lines[1])
e = toggleCard(obsidian, boards[1], 3, 1)
check("toggle done -> todo", e.lines[1] == "- [ ] T1 storage", e.lines[1])
e = toggleCard(obsidian, boards[1], 3, 2)
check("toggle plain -> done", e.lines[1] == "- [x] plain card", e.lines[1])
local cap = split("```kanban\n## A\n- [X] caps\n```")
e = toggleCard(cap, findBoards(cap)[1], 1, 1)
check("toggle capital X", e.lines[1] == "- [ ] caps", e.lines[1])

if failures > 0 then
    error(failures .. " kanban spec failure(s)")
end
print("kanban spec: all checks passed")
