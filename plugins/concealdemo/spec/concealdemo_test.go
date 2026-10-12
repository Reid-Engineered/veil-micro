package spec

import (
	"os"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestConcealdemoSpec runs concealdemo_spec.lua against concealdemo.lua in
// gopher-lua, the Lua implementation micro embeds.
func TestConcealdemoSpec(t *testing.T) {
	src, err := os.ReadFile("../concealdemo.lua")
	if err != nil {
		t.Fatal(err)
	}
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("PLUGIN_SRC", lua.LString(src))
	if err := L.DoFile("concealdemo_spec.lua"); err != nil {
		t.Fatal(err)
	}
}
