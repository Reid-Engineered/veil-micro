VERSION = "0.2.0"

-- vlinedemo: draws a box of virtual lines below the cursor line, to test the
-- veil virtual-lines rendering (T3/T4). Until those land, the decorations
-- are stored but nothing is drawn.
--
-- Commands:
--   vlinebox [text]   box below the cursor line, with per-cell colours
--   vlinehide         hide the cursor line itself (zero rows), box stays
--   vlineclear        remove everything this plugin added

local micro = import("micro")
local config = import("micro/config")
local utf8 = import("unicode/utf8")
local strings = import("strings")

local GROUP = "vlinedemo"

local function width(s)
    return utf8.RuneCountInString(s)
end

function vlinebox(bp, args)
    -- Command arguments arrive as a Go []string (Lua userdata, not a table),
    -- so join them with Go's strings.Join rather than table.concat.
    local text = strings.Join(args, " ")
    if text == "" then
        text = "virtual lines work"
    end
    local inner = width(text) + 2
    local rows = {
        "┌" .. string.rep("─", inner) .. "┐",
        "│ " .. text .. " │",
        "└" .. string.rep("─", inner) .. "┘",
    }
    local buf = bp.Buf
    local y = bp.Cursor.Y

    buf:SetOptionNative("conceal", true)
    buf:SetVirtualLines(y, rows, GROUP)
    -- per-cell colour: border in one style, text in another
    for row = 0, 2 do
        buf:StyleVirtualLine(y, row, 0, -1, "comment", GROUP)
    end
    buf:StyleVirtualLine(y, 1, 2, 2 + width(text), "special", GROUP)
    micro.InfoBar():Message("vlinedemo: box attached below line " .. (y + 1))
end

function vlinehide(bp, args)
    local y = bp.Cursor.Y
    bp.Buf:SetOptionNative("conceal", true)
    bp.Buf:ConcealLines(y, y, GROUP)
    micro.InfoBar():Message("vlinedemo: line " .. (y + 1) .. " hidden (shown while the cursor is on it)")
end

function vlineclear(bp, args)
    bp.Buf:ClearDecorations(GROUP)
    micro.InfoBar():Message("vlinedemo: cleared")
end

function init()
    config.MakeCommand("vlinebox", vlinebox, config.NoComplete)
    config.MakeCommand("vlinehide", vlinehide, config.NoComplete)
    config.MakeCommand("vlineclear", vlineclear, config.NoComplete)
    config.AddRuntimeFile("vlinedemo", config.RTHelp, "help/vlinedemo.md")
end
