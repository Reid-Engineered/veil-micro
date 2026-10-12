-- Tests for kanban.lua's micro-facing glue (callbacks, options, bindings),
-- with small stand-ins for micro's Lua modules and buffers. Runs under
-- gopher-lua via kanban_test.go, and under any Lua 5.1+ with PLUGIN_SRC set.

local failures = 0
local function check(name, cond, detail)
    if not cond then
        failures = failures + 1
        print("FAIL " .. name .. (detail and (": " .. tostring(detail)) or ""))
    end
end

------------------------------------------------------------------ stand-ins
local options = {}       -- global options, as config.GetGlobalOption sees them
local bindings = {}      -- key -> action, as bound by TryBindKey
local timers = {}
local curPane = nil

local function newBuf(path, text)
    local lines = {}
    for l in (text .. "\n"):gmatch("(.-)\n") do lines[#lines + 1] = l end
    local b = {
        AbsPath = path,
        Settings = { filetype = "markdown" },
        lines = lines,
        margins = {},   -- group -> margin passed to SetRevealMargin
        hidden = {},    -- group -> true while ConcealLines is in effect
    }
    function b:LinesNum() return #self.lines end
    function b:Line(y) return self.lines[y + 1] end
    function b:SetOptionNative(k, v) self.Settings[k] = v end
    function b:ClearDecorations(g) self.hidden[g] = nil end
    function b:SetRevealMode() return true end
    function b:SetRevealMargin(g, n) self.margins[g] = n; return true end
    function b:ConcealLines(a, z, g) self.hidden[g] = true end
    function b:SetVirtualLines(y, rows, g) self.rows = rows end
    function b:StyleVirtualLine() return true end
    return b
end

local modules = {
    micro = {
        After = function(_, f) table.insert(timers, f) end,
        CurPane = function() return curPane end,
        InfoBar = function() return { Message = function() end, Error = function() end } end,
    },
    ["micro/config"] = {
        RegisterGlobalOption = function(pl, name, default)
            if options[pl .. "." .. name] == nil then options[pl .. "." .. name] = default end
        end,
        GetGlobalOption = function(name) return options[name] end,
        MakeCommand = function() end,
        TryBindKey = function(k, v) bindings[k] = v; return true end,
        AddRuntimeFile = function() end,
        RTHelp = 0,
        NoComplete = nil,
    },
    ["micro/buffer"] = { Loc = function(x, y) return { X = x, Y = y } end },
    runtime = { GOOS = "linux" },
}
function import(name) return modules[name] end

local load_ = loadstring or load
assert(load_(PLUGIN_SRC))()

local function view(buf)
    return { Buf = buf, BufView = function() return { Width = 80 } end }
end

------------------------------------------------------------------ startup
preinit()
check("kanban.margin default is 1", options["kanban.margin"] == 1, options["kanban.margin"])
check("kanban.keys default is on", options["kanban.keys"] == true)
check("kanban.glyphpad default is 1", options["kanban.glyphpad"] == 1)

local board = newBuf("/b.md", "```kanban\n## A\n- [ ] one\n## B\n```")
curPane = view(board)
onBufferOpen(board) -- opened before init(), as at micro startup
init()
check("board drawn with default margin", board.margins["kanban:1"] == 1, board.margins["kanban:1"])
check("board hidden", board.hidden["kanban:1"] == true)

------------------------------------------------------------------ bindings
check("AltRight bound with fallback", bindings["AltRight"] == "lua:kanban.moveRight|EndOfLine", bindings["AltRight"])
check("AltUp falls back to MoveLinesUp", bindings["AltUp"] == "lua:kanban.moveUp|MoveLinesUp", bindings["AltUp"])
check("Alt-d bound", bindings["Alt-d"] == "lua:kanban.toggle")

------------------------------------------------------------------ set kanban.margin
-- micro sets the global value, then calls onBufferOptionChanged per buffer
options["kanban.margin"] = 0
onBufferOptionChanged(board, "kanban.margin", 1, 0)
check("set kanban.margin 0 updates an existing board", board.margins["kanban:1"] == 0, board.margins["kanban:1"])

options["kanban.margin"] = 3
local other = newBuf("/other.md", "```kanban\n## X\n- [ ] y\n```")
onBufferOpen(other)
onBufferOptionChanged(board, "kanban.margin", 0, 3)
onBufferOptionChanged(other, "kanban.margin", 0, 3)
check("all open boards follow the setting", board.margins["kanban:1"] == 3 and other.margins["kanban:1"] == 3)

------------------------------------------------------------------ set kanban.glyphpad
local function cardRow(buf)
    for _, r in ipairs(buf.rows or {}) do
        if string.find(r, "☐", 1, true) then return r end
    end
end
check("glyphpad default 1 pads the mark", string.find(cardRow(board) or "", "☐  one", 1, true) ~= nil, cardRow(board))
options["kanban.glyphpad"] = 0
onBufferOptionChanged(board, "kanban.glyphpad", 1, 0)
check("set kanban.glyphpad 0 redraws open boards", string.find(cardRow(board) or "", "☐ one", 1, true) ~= nil, cardRow(board))
options["kanban.glyphpad"] = 1
onBufferOptionChanged(board, "kanban.glyphpad", 0, 1)

------------------------------------------------------------------ deinit / filetype
deinit()
check("deinit clears boards", board.hidden["kanban:1"] == nil)
init()
check("re-init redraws", board.hidden["kanban:1"] == true)

onBufferOptionChanged(board, "filetype", "markdown", "text")
check("leaving markdown clears the board", board.hidden["kanban:1"] == nil)

------------------------------------------------------------------ keys off
bindings = {}
options["kanban.keys"] = false
init()
check("kanban.keys off binds nothing", next(bindings) == nil)

if failures > 0 then
    error(failures .. " kanban glue spec failure(s)")
end
print("kanban glue spec: all checks passed")
