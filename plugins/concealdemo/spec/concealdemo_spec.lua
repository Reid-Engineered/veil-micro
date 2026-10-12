-- Tests for concealdemo with stand-in micro modules. Runs under gopher-lua via
-- concealdemo_test.go, and under any Lua 5.1+ with PLUGIN_SRC set.
-- Lives in spec/ because micro loads every top-level .lua file in a plugin dir.

local failures = 0
local function check(name, cond, detail)
    if not cond then
        failures = failures + 1
        print("FAIL " .. name .. (detail and (": " .. tostring(detail)) or ""))
    end
end

local options, timers, curPane = {}, {}, nil
local function newBuf(path, lines)
    local b = { AbsPath = path, Settings = { filetype = "markdown" }, lines = lines, deco = {} }
    function b:LinesNum() return #self.lines end
    function b:Line(y) return self.lines[y + 1] end
    function b:ClearDecorations() self.deco = {} end
    function b:AddConceal(y, s, e, r) table.insert(self.deco, { y = y, s = s, e = e, r = r }); return true end
    function b:SetOptionNative(k, v) self.Settings[k] = v end
    return b
end
local modules = {
    micro = { After = function(_, f) table.insert(timers, f) end, CurPane = function() return curPane end },
    ["micro/config"] = {
        AddRuntimeFile = function() end, RTHelp = 0,
        RegisterGlobalOption = function(pl, name, default)
            if options[pl .. "." .. name] == nil then options[pl .. "." .. name] = default end
        end,
        GetGlobalOption = function(name) return options[name] end,
    },
}
function import(name) return modules[name] end

local load_ = loadstring or load
assert(load_(PLUGIN_SRC))()

-- Started disabled (concealdemo=false in settings.json): micro skips preinit();
-- `set concealdemo on` then calls only init(), and markdown opens afterwards.
init()
check("init alone registers glyphpad with default 1", options["concealdemo.glyphpad"] == 1, options["concealdemo.glyphpad"])
local b = newBuf("/a.md", { "- [x] done", "- [ ] todo", "text" })
curPane = { Buf = b }
onBufferOpen(b)
check("two conceals", #b.deco == 2, #b.deco)
check("done glyph padded", b.deco[1] and b.deco[1].r == "✔ ", b.deco[1] and b.deco[1].r)
check("todo glyph padded", b.deco[2] and b.deco[2].r == "☐ ", b.deco[2] and b.deco[2].r)
check("conceal range is just '- [x]'", b.deco[1] and b.deco[1].s == 0 and b.deco[1].e == 5)

-- setting changes redraw immediately (micro calls this per open buffer)
options["concealdemo.glyphpad"] = 0
onBufferOptionChanged(b, "concealdemo.glyphpad", 1, 0)
check("glyphpad 0 applies immediately", b.deco[1] and b.deco[1].r == "✔", b.deco[1] and b.deco[1].r)

-- odd values are normalized instead of breaking string.rep
for _, v in ipairs({ -3, 2.6, 1000 }) do
    options["concealdemo.glyphpad"] = v
    onBufferOptionChanged(b, "concealdemo.glyphpad", 0, v)
    local r = b.deco[1] and b.deco[1].r or ""
    local spaces = #r - #"✔"
    check("glyphpad " .. v .. " normalized", spaces >= 0 and spaces <= 8, spaces)
end

-- registering again (preinit after init, or a second init) keeps the user's value
options["concealdemo.glyphpad"] = 3
preinit()
init()
check("re-registering keeps the value", options["concealdemo.glyphpad"] == 3, options["concealdemo.glyphpad"])

if failures > 0 then
    error(failures .. " concealdemo spec failure(s)")
end
print("concealdemo spec: all checks passed")
