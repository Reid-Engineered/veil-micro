VERSION = "0.1.0"

-- concealdemo: renders markdown task checkboxes as glyphs using the veil
-- decorations API.
--
--   - [x] done   ->  ✔ done
--   - [ ] todo   ->  ☐ todo
--
-- It turns on the `conceal` option for markdown buffers only, and rescans
-- the current buffer shortly after every edit (debounced), because an edit
-- drops the conceals of the lines it touches (see docs/API.md).

local micro = import("micro")
local config = import("micro/config")

local GROUP = "concealdemo"
local DELAY_NS = 150 * 1000 * 1000 -- 150 ms, micro.After takes nanoseconds

local generation = 0

local function isMarkdown(buf)
    return buf.Settings["filetype"] == "markdown"
end

-- Conceal "- [x]" / "* [ ]" / "+ [X]" (after any indentation) on every line.
-- The matched prefix is plain ASCII, so Lua's byte offsets equal micro's
-- character columns here. A general plugin would have to convert.
function scan(buf)
    if buf == nil or not isMarkdown(buf) then
        return
    end
    buf:ClearDecorations(GROUP)
    for y = 0, buf:LinesNum() - 1 do
        local line = buf:Line(y)
        local s, e, mark = string.find(line, "^%s*[-*+] %[([ xX])%]")
        if s ~= nil then
            local indent = string.find(line, "[-*+]") - 1
            if mark == " " then
                buf:AddConceal(y, indent, e, "☐", "special", GROUP)
            else
                buf:AddConceal(y, indent, e, "✔", "comment", GROUP)
            end
        end
    end
end

local function scheduleRescan()
    generation = generation + 1
    local mine = generation
    micro.After(DELAY_NS, function()
        if mine == generation then
            scan(micro.CurPane().Buf)
        end
    end)
end

function init()
    config.AddRuntimeFile("concealdemo", config.RTHelp, "help/concealdemo.md")
end

function onBufferOpen(buf)
    if isMarkdown(buf) then
        buf:SetOptionNative("conceal", true)
        scan(buf)
    end
end

-- The filetype can change after the buffer opens (e.g. `set filetype markdown`).
function onBufferOptionChanged(buf, option, old, new)
    if option == "filetype" and new == "markdown" then
        buf:SetOptionNative("conceal", true)
        scan(buf)
    end
end

-- Must return true: returning false cancels the edit.
function onBeforeTextEvent(sbuf, ev)
    scheduleRescan()
    return true
end

-- Undo and redo replay edits without onBeforeTextEvent.
function onUndo(bp)
    scheduleRescan()
    return true
end

function onRedo(bp)
    scheduleRescan()
    return true
end
