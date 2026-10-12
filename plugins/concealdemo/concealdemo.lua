VERSION = "0.3.0"

-- concealdemo: renders markdown task checkboxes as glyphs using the veil
-- decorations API.
--
--   - [x] done   ->  ✔ done
--   - [ ] todo   ->  ☐ todo
--
-- It turns on the `conceal` option for markdown buffers only, and rescans a
-- buffer shortly after every edit to it (debounced per buffer), because an
-- edit drops the conceals of the lines it touches (see docs/API.md).

local micro = import("micro")
local config = import("micro/config")

local GROUP = "concealdemo"
local DELAY_NS = 150 * 1000 * 1000 -- 150 ms, micro.After takes nanoseconds

-- `set concealdemo off` calls deinit() but keeps delivering callbacks, so
-- every entry point checks this flag.
local enabled = false

-- Buffers seen in onBufferOpen. Lua can't list micro's open buffers, and
-- onBeforeTextEvent only receives the shared buffer, so we keep our own list
-- to map an edit back to a *Buffer.
local tracked = {}

-- Per-buffer debounce: key -> generation of the latest scheduled rescan.
local pending = {}

local function isMarkdown(buf)
    return buf.Settings["filetype"] == "markdown"
end

local function key(buf)
    return buf.AbsPath
end

-- Conceal "- [x]" / "* [ ]" / "+ [X]" (after any indentation) on every line.
-- The matched prefix is plain ASCII, so Lua's byte offsets equal micro's
-- character columns here. A general plugin would have to convert.
function scan(buf)
    if buf == nil then
        return
    end
    buf:ClearDecorations(GROUP)
    if not enabled or not isMarkdown(buf) then
        return
    end
    for y = 0, buf:LinesNum() - 1 do
        local line = buf:Line(y)
        local s, e, mark = string.find(line, "^%s*[-*+] %[([ xX])%]")
        if s ~= nil then
            local indent = string.find(line, "[-*+]") - 1
            -- Extra spaces after the glyph: fonts without ✔/☐ (e.g. Cascadia Mono)
            -- get them from a fallback font that draws them ~2 cells wide, which
            -- covers the following space and makes the text look glued on.
            local gap = string.rep(" ", config.GetGlobalOption("concealdemo.glyphpad") or 0)
            if mark == " " then
                buf:AddConceal(y, indent, e, "☐" .. gap, "special", GROUP)
            else
                buf:AddConceal(y, indent, e, "✔" .. gap, "comment", GROUP)
            end
        end
    end
end

-- Deduplicate by path: two Lua handles to one Go buffer may not compare
-- equal, and splits of one file share their decorations anyway. Unnamed
-- buffers can't be told apart this way, so each is kept.
local function track(buf)
    if buf.AbsPath ~= "" then
        for i, b in ipairs(tracked) do
            if b.AbsPath == buf.AbsPath then
                tracked[i] = buf -- newest handle: a reopened file gets new text storage
                return
            end
        end
    end
    table.insert(tracked, buf)
end

-- Find the *Buffer an edit belongs to. Edits are delivered synchronously, so
-- the current pane's buffer is right almost always; check it against the
-- edited file's path, then fall back to our own list. Splits of one file
-- share their text and decorations, so any matching buffer will do.
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
    local k = key(buf)
    local gen = (pending[k] or 0) + 1
    pending[k] = gen
    micro.After(DELAY_NS, function()
        if enabled and pending[k] == gen then
            scan(buf)
        end
    end)
end

-- Enabling never registers: init() iterates `tracked`, and registering
-- here would append to the list being iterated (an unnamed buffer can't be
-- deduplicated, so the loop would never end).
local function enableFor(buf)
    if enabled and isMarkdown(buf) then
        buf:SetOptionNative("conceal", true)
        scan(buf)
    end
end

function preinit()
    -- extra spaces after ✔/☐ for fonts that draw them two cells wide
    config.RegisterGlobalOption("concealdemo", "glyphpad", 1)
end

function init()
    enabled = true
    config.AddRuntimeFile("concealdemo", config.RTHelp, "help/concealdemo.md")
    -- Startup opens the initial buffers before init(), and `set concealdemo on`
    -- calls init() again: restore glyphs on buffers we already know about.
    -- Iterate a snapshot so nothing that runs here can grow the loop.
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
        b:ClearDecorations(GROUP)
    end
end

-- The only place buffers are registered, so each open adds one entry.
function onBufferOpen(buf)
    track(buf)
    enableFor(buf)
end

-- Entering markdown turns the glyphs on; leaving it removes them.
function onBufferOptionChanged(buf, option, old, new)
    -- micro calls this for every open buffer after a global `set`
    if option == "concealdemo.glyphpad" then
        enableFor(buf)
        return
    end
    if option ~= "filetype" then
        return
    end
    if new == "markdown" then
        enableFor(buf)
    elseif old == "markdown" then
        buf:ClearDecorations(GROUP)
    end
end

-- Must return true: returning false cancels the edit.
function onBeforeTextEvent(sbuf, ev)
    if enabled then
        scheduleRescan(bufferFor(sbuf))
    end
    return true
end

-- Undo and redo replay edits without onBeforeTextEvent.
function onUndo(bp)
    scheduleRescan(bp.Buf)
    return true
end

function onRedo(bp)
    scheduleRescan(bp.Buf)
    return true
end
