package display

import "github.com/micro-editor/micro/v2/internal/buffer"

// concealAt is the shared hook reserved for T2's conceal implementation.
func (w *BufWindow) concealAt(line, x int) *buffer.Conceal {
	return nil
}
