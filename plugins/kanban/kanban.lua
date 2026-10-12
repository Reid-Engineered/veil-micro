VERSION = "0.2.0"

-- kanban: draws markdown kanban boards inline, using the veil decorations API.
--
-- A board is either
--   * a whole Obsidian Kanban file (front matter containing `kanban-plugin:`), or
--   * a fenced block: a line ```kanban ... closed by ```.
-- Inside a board, `## Heading` starts a column and `- [ ] text` / `- [x] text`
-- (or a plain `- text`) is a card. Indented lines under a card belong to it.
--
-- When no cursor is inside a board, its raw lines are hidden (ConcealLines) and
-- the board is drawn as side-by-side columns with virtual lines. Moving the
-- cursor into the board (or clicking it) reveals the raw markdown, so all
-- editing happens on real text.
--
-- Keys, inside a board with the cursor on a card (elsewhere the key keeps its
-- normal job, through a micro action chain fallback):
--   Alt-Right / Alt-Left   move the card to the next / previous column
--   Alt-Up / Alt-Down      move the card up / down within its column
--   Alt-d                  toggle done
-- Keys you bound yourself in bindings.json always win; `set kanban.keys off`
-- skips binding entirely. Commands: `kanban right|left|up|down|toggle|redraw`.
--
-- Everything up to the "micro glue" section is plain Lua with no micro calls,
-- so it can be tested outside the editor (see spec/kanban_spec.lua).

------------------------------------------------------------------------------
-- UTF-8 helpers (Lua 5.1 has no utf8 library; count non-continuation bytes)
------------------------------------------------------------------------------

local function ulen(s)
    local n = 0
    for i = 1, #s do
        local b = string.byte(s, i)
        if b < 128 or b >= 192 then
            n = n + 1
        end
    end
    return n
end

-- first `n` characters of s
local function uhead(s, n)
    if n <= 0 then
        return ""
    end
    local count = 0
    for i = 1, #s do
        local b = string.byte(s, i)
        if b < 128 or b >= 192 then
            count = count + 1
            if count > n then
                return string.sub(s, 1, i - 1)
            end
        end
    end
    return s
end

-- characters after the first `n`
local function utail(s, n)
    return string.sub(s, #uhead(s, n) + 1)
end

local function pad(s, width)
    local l = ulen(s)
    if l >= width then
        return uhead(s, width)
    end
    return s .. string.rep(" ", width - l)
end

-- truncate with an ellipsis if longer than width
local function fit(s, width)
    if ulen(s) <= width then
        return s
    end
    if width <= 1 then
        return uhead(s, width)
    end
    return uhead(s, width - 1) .. "…"
end

-- Greedy word wrap into lines of at most `width` characters. Words longer
-- than the width are split.
local function wrap(text, width)
    local lines = {}
    local cur = ""
    for word in string.gmatch(text, "%S+") do
        while ulen(word) > width do
            if cur ~= "" then
                table.insert(lines, cur)
                cur = ""
            end
            table.insert(lines, uhead(word, width))
            word = utail(word, width)
        end
        if cur == "" then
            cur = word
        elseif ulen(cur) + 1 + ulen(word) <= width then
            cur = cur .. " " .. word
        else
            table.insert(lines, cur)
            cur = word
        end
    end
    if cur ~= "" or #lines == 0 then
        table.insert(lines, cur)
    end
    return lines
end

------------------------------------------------------------------------------
-- Parsing. `lines` is a 1-based array of strings; every line number stored in
-- the model is 0-based, like micro's.
------------------------------------------------------------------------------

local function isBlank(s)
    return string.find(s, "^%s*$") ~= nil
end

local function isIndented(s)
    return string.find(s, "^[ \t]") ~= nil
end

-- Parse columns and cards in lines [from, to] (0-based, inclusive).
local function parseColumns(lines, from, to)
    local columns = {}
    local col, card = nil, nil
    for y = from, to do
        local s = lines[y + 1]
        local title = string.match(s, "^##%s+(.-)%s*$")
        if title ~= nil and string.sub(s, 1, 3) ~= "###" then
            col = { title = title, line = y, cards = {} }
            table.insert(columns, col)
            card = nil
        elseif col ~= nil then
            local mark, text = string.match(s, "^[-*+] %[([ xX])%]%s?(.*)$")
            local plain = nil
            if mark == nil then
                plain = string.match(s, "^[-*+]%s+(.*)$")
            end
            if mark ~= nil or plain ~= nil then
                local done = nil
                if mark ~= nil then
                    done = (mark ~= " ")
                else
                    text = plain
                end
                card = { text = text, done = done, line = y, endLine = y }
                table.insert(col.cards, card)
            elseif card ~= nil and isIndented(s) and not isBlank(s) then
                card.endLine = y
                card.text = card.text .. " " .. string.match(s, "^%s*(.-)%s*$")
            else
                card = nil
            end
        end
    end
    return columns
end

-- Last line of a board's own content: the last column heading or card line.
local function contentEnd(columns)
    local last = nil
    for _, c in ipairs(columns) do
        last = c.line
        for _, k in ipairs(c.cards) do
            last = k.endLine
        end
    end
    return last
end

-- If line opens a code fence, return the fence run (e.g. "````") and the
-- first word of its info string.
function fenceOpen(line)
    local run, rest = string.match(line, "^%s*(```+)(.*)$")
    if run == nil then
        run, rest = string.match(line, "^%s*(~~~+)(.*)$")
    elseif string.find(rest, "`", 1, true) then
        return nil -- backtick fences can't have backticks in the info string
    end
    if run == nil then
        return nil
    end
    return run, (string.match(rest, "^%s*(%S*)") or "")
end

-- Does line close a fence opened with `fence`? Same character, at least as
-- long, nothing else on the line.
function fenceCloses(line, fence)
    local ch = string.sub(fence, 1, 1)
    local run = string.match(line, "^%s*(" .. (ch == "`" and "`" or "~") .. "+)%s*$")
    return run ~= nil and #run >= #fence
end

-- Find every board in a file.
-- Returns a list of { kind, first, last, columns } where [first, last] are the
-- 0-based lines to hide (fences included for fenced boards).
function findBoards(lines)
    local boards = {}
    local n = #lines

    -- Obsidian Kanban file: front matter with a kanban-plugin key
    if n > 0 and string.find(lines[1], "^%-%-%-%s*$") then
        local isKanban, fmEnd = false, nil
        for i = 2, n do
            if string.find(lines[i], "^%-%-%-%s*$") then
                fmEnd = i - 1 -- 0-based line of the closing ---
                break
            end
            if string.find(lines[i], "^kanban%-plugin:") then
                isKanban = true
            end
        end
        if isKanban and fmEnd ~= nil then
            local stop = n - 1
            for y = fmEnd + 1, n - 1 do
                if string.find(lines[y + 1], "^%%%%%s*kanban:settings") then
                    stop = y - 1
                    break
                end
            end
            local columns = parseColumns(lines, fmEnd + 1, stop)
            if #columns > 0 then
                table.insert(boards, {
                    kind = "file",
                    first = columns[1].line,
                    last = contentEnd(columns),
                    columns = columns,
                })
            end
            return boards
        end
    end

    -- fenced ```kanban blocks. Every code fence is tracked (CommonMark rules:
    -- ``` or ~~~ runs of 3+, closed by a run of the same character at least
    -- as long), so a ```kanban example inside another code fence stays text.
    local y = 0
    while y < n do
        local fence, info = fenceOpen(lines[y + 1])
        if fence == nil then
            y = y + 1
        else
            local close = nil
            for z = y + 1, n - 1 do
                if fenceCloses(lines[z + 1], fence) then
                    close = z
                    break
                end
            end
            if close == nil then
                break -- unterminated fence runs to the end of the file
            end
            if info == "kanban" then
                local columns = parseColumns(lines, y + 1, close - 1)
                if #columns > 0 then
                    table.insert(boards, { kind = "fence", first = y, last = close, columns = columns })
                end
            end
            y = close + 1
        end
    end
    return boards
end

-- The card containing line y, as board, column index, card index.
function cardAt(boards, y)
    for _, b in ipairs(boards) do
        if y >= b.first and y <= b.last then
            for ci, c in ipairs(b.columns) do
                for ki, k in ipairs(c.cards) do
                    if y >= k.line and y <= k.endLine then
                        return b, ci, ki
                    end
                end
            end
            return b, nil, nil
        end
    end
    return nil, nil, nil
end

------------------------------------------------------------------------------
-- Layout: board -> virtual rows + style spans
------------------------------------------------------------------------------

local MIN_CONTENT, MAX_CONTENT = 8, 28

-- Style names are colorscheme groups present in micro's default themes.
local STYLE_BORDER = "comment"
local STYLE_TITLE = "special"
local STYLE_TODO_MARK = "special"
local STYLE_DONE = "comment"

-- Returns rows (list of strings) and spans (list of {row, s, e, style}, row
-- 0-based, columns in characters, e exclusive or -1 for end of row).
function layout(board, width)
    local cols = board.columns
    local n = #cols
    -- each column: │ + space + content + space ; plus one closing │
    local content = math.floor((width - 1) / n) - 3
    if content < MIN_CONTENT then
        content = MIN_CONTENT
    elseif content > MAX_CONTENT then
        content = MAX_CONTENT
    end
    local inner = content + 2

    -- per-column cell lines: { text = ..., kind = "todo"|"done"|"plain"|"cont-..." }
    local cells, height = {}, 0
    for i, c in ipairs(cols) do
        local list = {}
        for _, k in ipairs(c.cards) do
            local mark, kind = "• ", "plain"
            if k.done == true then
                mark, kind = "✔ ", "done"
            elseif k.done == false then
                mark, kind = "☐ ", "todo"
            end
            local wrapped = wrap(k.text, content - 2)
            for j, w in ipairs(wrapped) do
                if j == 1 then
                    table.insert(list, { text = mark .. w, kind = kind, first = true })
                else
                    table.insert(list, { text = "  " .. w, kind = kind, first = false })
                end
            end
        end
        cells[i] = list
        if #list > height then
            height = #list
        end
    end

    local function rule(l, m, r)
        local parts = {}
        for i = 1, n do
            parts[i] = string.rep("─", inner)
        end
        return l .. table.concat(parts, m) .. r
    end

    local rows, spans = {}, {}
    local function addRow(s)
        table.insert(rows, s)
        table.insert(spans, { #rows - 1, 0, -1, STYLE_BORDER })
    end
    -- character column where column i's content starts
    local function x0(i)
        return (i - 1) * (inner + 1) + 2
    end

    addRow(rule("┌", "┬", "┐"))

    local header = {}
    for i, c in ipairs(cols) do
        header[i] = " " .. pad(fit(c.title .. " (" .. #c.cards .. ")", content), content) .. " "
    end
    addRow("│" .. table.concat(header, "│") .. "│")
    for i = 1, n do
        table.insert(spans, { #rows - 1, x0(i), x0(i) + content, STYLE_TITLE })
    end

    addRow(rule("├", "┼", "┤"))

    for r = 1, height do
        local parts = {}
        for i = 1, n do
            local cell = cells[i][r]
            parts[i] = " " .. pad(cell and cell.text or "", content) .. " "
        end
        addRow("│" .. table.concat(parts, "│") .. "│")
        local row = #rows - 1
        for i = 1, n do
            local cell = cells[i][r]
            if cell ~= nil then
                local s = x0(i)
                if cell.kind == "done" then
                    table.insert(spans, { row, s, s + content, STYLE_DONE })
                else
                    -- card text in the default style, todo box highlighted
                    table.insert(spans, { row, s, s + content, "" })
                    if cell.kind == "todo" and cell.first then
                        table.insert(spans, { row, s, s + 1, STYLE_TODO_MARK })
                    end
                end
            end
        end
    end

    addRow(rule("└", "┴", "┘"))
    return rows, spans
end

------------------------------------------------------------------------------
-- Edits, computed on line arrays. Each returns a minimal replacement
-- { lo, hi, lines, cursor } meaning: replace 0-based lines [lo, hi] with
-- `lines` (same count) and put the cursor on line `cursor`; or nil, reason.
------------------------------------------------------------------------------

-- Move the card at (ci, ki) of `board` to the end of column ci + dir.
function moveCard(lines, board, ci, ki, dir)
    local target = board.columns[ci + dir]
    if target == nil then
        if dir > 0 then
            return nil, "already in the last column"
        end
        return nil, "already in the first column"
    end
    local card = board.columns[ci].cards[ki]
    local first, last = card.line, card.endLine
    local count = last - first + 1

    -- insertion point, as an original 0-based line number: after the target's
    -- last card, or after its heading (and one blank line, Obsidian style)
    local ins
    if #target.cards > 0 then
        ins = target.cards[#target.cards].endLine + 1
    else
        ins = target.line + 1
        if lines[ins + 1] ~= nil and isBlank(lines[ins + 1]) and ins <= board.last then
            ins = ins + 1
        end
    end

    local block = {}
    for y = first, last do
        table.insert(block, lines[y + 1])
    end

    local out = {}
    for y = 0, #lines - 1 do
        if y == ins then
            for _, b in ipairs(block) do
                table.insert(out, b)
            end
        end
        if y < first or y > last then
            table.insert(out, lines[y + 1])
        end
    end
    if ins >= #lines then
        for _, b in ipairs(block) do
            table.insert(out, b)
        end
    end

    local cursor
    if ins > last then
        cursor = ins - count
    else
        cursor = ins
    end

    local lo = math.min(first, ins)
    local hi = math.max(last, ins - 1)
    local slice = {}
    for y = lo, hi do
        table.insert(slice, out[y + 1])
    end
    return { lo = lo, hi = hi, lines = slice, cursor = cursor }
end

-- Swap the card at (ci, ki) with its neighbour above (dir = -1) or below
-- (dir = 1) in the same column. Lines between the two cards (e.g. a blank
-- line) stay between them.
function reorderCard(lines, board, ci, ki, dir)
    local cards = board.columns[ci].cards
    local other = cards[ki + dir]
    if other == nil then
        if dir < 0 then
            return nil, "already the first card in this column"
        end
        return nil, "already the last card in this column"
    end
    local upper, lower = cards[ki], other
    if dir < 0 then
        upper, lower = other, cards[ki]
    end
    local out = {}
    for y = lower.line, lower.endLine do
        table.insert(out, lines[y + 1])
    end
    for y = upper.endLine + 1, lower.line - 1 do
        table.insert(out, lines[y + 1])
    end
    for y = upper.line, upper.endLine do
        table.insert(out, lines[y + 1])
    end
    local lo, hi = upper.line, lower.endLine
    local cursor
    if dir < 0 then
        cursor = lo -- the moved card now comes first
    else
        cursor = lo + (lower.endLine - lower.line + 1) + (lower.line - upper.endLine - 1)
    end
    return { lo = lo, hi = hi, lines = out, cursor = cursor }
end

-- Flip a card's checkbox; plain cards (no checkbox) get one.
function toggleCard(lines, board, ci, ki)
    local card = board.columns[ci].cards[ki]
    local s = lines[card.line + 1]
    local new
    if card.done == true then
        new = string.gsub(s, "^([-*+] )%[[xX]%]", "%1[ ]", 1)
    elseif card.done == false then
        new = string.gsub(s, "^([-*+] )%[ %]", "%1[x]", 1)
    else
        new = string.gsub(s, "^([-*+])%s+", "%1 [x] ", 1)
    end
    return { lo = card.line, hi = card.line, lines = { new }, cursor = card.line }
end

-- Turn an edit into one replacement per changed line. Moves and toggles never
-- change the line count, so every delta is single-line. micro's MultipleReplace
-- applies all deltas as ONE undo event; a plain Replace would be a Remove plus
-- an Insert, which undo may split across its one-second grouping, and micro's
-- replace event assumes single-line text when it records the undo range.
-- Returns a list of { y, old, new } (0-based line numbers).
function editLines(lines, edit)
    local out = {}
    for i, new in ipairs(edit.lines) do
        local y = edit.lo + i - 1
        local old = lines[y + 1]
        if old ~= new then
            table.insert(out, { y = y, old = old, new = new })
        end
    end
    return out
end

-- Apply an edit to a micro buffer as a single MultipleReplace (one undo
-- event), see editLines. `loc` builds a buffer.Loc; it's a parameter so tests
-- can call this without the micro modules.
function applyEdit(buf, lines, edit, loc)
    local deltas = {}
    for _, d in ipairs(editLines(lines, edit)) do
        -- End past the line's last character is clamped by micro; byte length
        -- is always >= the character count.
        table.insert(deltas, { Text = d.new, Start = loc(0, d.y), End = loc(#d.old, d.y) })
    end
    if #deltas > 0 then
        buf:MultipleReplace(deltas)
    end
end

------------------------------------------------------------------------------
-- micro glue. Nothing above this line calls into micro.
------------------------------------------------------------------------------

if import == nil then
    return -- loaded by a test harness without micro
end

local micro = import("micro")
local config = import("micro/config")
local buffer = import("micro/buffer")

local DELAY_NS = 150 * 1000 * 1000 -- micro.After takes nanoseconds
local DEFAULT_WIDTH = 80

local enabled = false
local tracked = {}       -- buffers seen in onBufferOpen (see concealdemo for why)
local pending = {}       -- per-buffer debounce generation
-- Highest number of boards ever drawn in any buffer. Cleanup clears groups
-- kanban:1..maxGroups, so it never depends on knowing which buffer drew what
-- (unnamed buffers all share AbsPath "", so per-path bookkeeping is wrong).
local maxGroups = 0

local function isMarkdown(buf)
    return buf.Settings["filetype"] == "markdown"
end

local function bufLines(buf)
    local lines = {}
    for y = 0, buf:LinesNum() - 1 do
        lines[y + 1] = buf:Line(y)
    end
    return lines
end

local function clearBoards(buf)
    for i = 1, maxGroups do
        buf:ClearDecorations("kanban:" .. i)
    end
end

local function widthFor(buf)
    local cur = micro.CurPane()
    if cur ~= nil and cur.Buf ~= nil and cur.Buf.AbsPath == buf.AbsPath then
        local w = cur:BufView().Width
        if w ~= nil and w > 0 then
            return w
        end
    end
    return DEFAULT_WIDTH
end

function scan(buf)
    if buf == nil then
        return
    end
    clearBoards(buf)
    if not enabled or not isMarkdown(buf) then
        return
    end
    local boards = findBoards(bufLines(buf))
    if #boards == 0 then
        return
    end
    buf:SetOptionNative("conceal", true)
    local width = widthFor(buf)
    for i, b in ipairs(boards) do
        local g = "kanban:" .. i
        buf:SetRevealMode(g, "group")
        buf:ConcealLines(b.first, b.last, g)
        local rows, spans = layout(b, width)
        buf:SetVirtualLines(b.first, rows, g)
        for _, sp in ipairs(spans) do
            buf:StyleVirtualLine(b.first, sp[1], sp[2], sp[3], sp[4], g)
        end
    end
    if #boards > maxGroups then
        maxGroups = #boards
    end
end

local function track(buf)
    if buf.AbsPath ~= "" then
        for i, b in ipairs(tracked) do
            if b.AbsPath == buf.AbsPath then
                tracked[i] = buf
                return
            end
        end
    end
    table.insert(tracked, buf)
end

local function bufferFor(sbuf)
    local cur = micro.CurPane()
    if cur ~= nil and cur.Buf ~= nil and cur.Buf.AbsPath == sbuf.AbsPath then
        return cur.Buf
    end
    if sbuf.AbsPath ~= "" then
        for _, b in ipairs(tracked) do
            if b.AbsPath == sbuf.AbsPath then
                return b
            end
        end
    end
    if cur ~= nil then
        return cur.Buf
    end
    return nil
end

local function scheduleRescan(buf)
    if not enabled or buf == nil or not isMarkdown(buf) then
        return
    end
    local k = buf.AbsPath
    local gen = (pending[k] or 0) + 1
    pending[k] = gen
    micro.After(DELAY_NS, function()
        if enabled and pending[k] == gen then
            scan(buf)
        end
    end)
end

local function apply(bp, lines, edit)
    applyEdit(bp.Buf, lines, edit, buffer.Loc)
    bp.Cursor:GotoLoc(buffer.Loc(0, edit.cursor))
    bp:Relocate()
end

local function withCard(bp, fn)
    if not enabled or not isMarkdown(bp.Buf) then
        return false
    end
    local lines = bufLines(bp.Buf)
    local board, ci, ki = cardAt(findBoards(lines), bp.Cursor.Y)
    if board == nil or ci == nil then
        -- not on a card (outside a board, or on a heading): returning false
        -- lets the key's fallback action in the binding chain run instead
        return false
    end
    local edit, err = fn(lines, board, ci, ki)
    if edit == nil then
        micro.InfoBar():Message("kanban: " .. err)
        return true
    end
    apply(bp, lines, edit)
    return true
end

function moveRight(bp)
    return withCard(bp, function(l, b, ci, ki) return moveCard(l, b, ci, ki, 1) end)
end

function moveLeft(bp)
    return withCard(bp, function(l, b, ci, ki) return moveCard(l, b, ci, ki, -1) end)
end

function moveUp(bp)
    return withCard(bp, function(l, b, ci, ki) return reorderCard(l, b, ci, ki, -1) end)
end

function moveDown(bp)
    return withCard(bp, function(l, b, ci, ki) return reorderCard(l, b, ci, ki, 1) end)
end

function toggle(bp)
    return withCard(bp, toggleCard)
end

local function onCard(bp)
    local board, ci = cardAt(findBoards(bufLines(bp.Buf)), bp.Cursor.Y)
    return board ~= nil and ci ~= nil
end

local function command(bp, args)
    local which = ""
    if #args > 0 then
        which = args[1]
    end
    if which ~= "redraw" and which ~= "" and not onCard(bp) then
        micro.InfoBar():Message("kanban: put the cursor on a card")
        return
    end
    if which == "right" then
        moveRight(bp)
    elseif which == "left" then
        moveLeft(bp)
    elseif which == "up" then
        moveUp(bp)
    elseif which == "down" then
        moveDown(bp)
    elseif which == "toggle" then
        toggle(bp)
    elseif which == "redraw" then
        scan(bp.Buf)
    else
        micro.InfoBar():Error("usage: kanban right|left|up|down|toggle|redraw")
    end
end

-- Each key runs the kanban action first; outside a card it returns false and
-- the chain (`|`) falls back to micro's default for that key on this platform.
-- TryBindKey with overwrite = false never replaces a binding from the user's
-- bindings.json, and plugin bindings are not written to that file.
function bindKeys()
    local runtime = import("runtime")
    local mac = runtime.GOOS == "darwin"
    local fallback = {
        AltRight = mac and "WordRight" or "EndOfLine",
        AltLeft = mac and "WordLeft" or "StartOfTextToggle",
        AltUp = "MoveLinesUp",
        AltDown = "MoveLinesDown",
    }
    config.TryBindKey("AltRight", "lua:kanban.moveRight|" .. fallback.AltRight, false)
    config.TryBindKey("AltLeft", "lua:kanban.moveLeft|" .. fallback.AltLeft, false)
    config.TryBindKey("AltUp", "lua:kanban.moveUp|" .. fallback.AltUp, false)
    config.TryBindKey("AltDown", "lua:kanban.moveDown|" .. fallback.AltDown, false)
    config.TryBindKey("Alt-d", "lua:kanban.toggle", false)
end

local function enableFor(buf)
    if enabled and isMarkdown(buf) then
        scan(buf)
    end
end

function preinit()
    -- registered in preinit so a value in settings.json is applied before init()
    config.RegisterGlobalOption("kanban", "keys", true)
end

function init()
    enabled = true
    config.MakeCommand("kanban", command, config.NoComplete)
    if config.GetGlobalOption("kanban.keys") then
        bindKeys()
    end
    config.AddRuntimeFile("kanban", config.RTHelp, "help/kanban.md")
    local snapshot = {}
    for i, b in ipairs(tracked) do
        snapshot[i] = b
    end
    for _, b in ipairs(snapshot) do
        enableFor(b)
    end
end

function deinit()
    enabled = false
    pending = {}
    for _, b in ipairs(tracked) do
        clearBoards(b)
    end
end

function onBufferOpen(buf)
    track(buf)
    enableFor(buf)
end

function onBufferOptionChanged(buf, option, old, new)
    if option ~= "filetype" then
        return
    end
    if new == "markdown" then
        enableFor(buf)
    elseif old == "markdown" then
        clearBoards(buf)
    end
end

-- Must return true: returning false cancels the edit.
function onBeforeTextEvent(sbuf, ev)
    if enabled then
        scheduleRescan(bufferFor(sbuf))
    end
    return true
end

function onUndo(bp)
    scheduleRescan(bp.Buf)
    return true
end

function onRedo(bp)
    scheduleRescan(bp.Buf)
    return true
end
