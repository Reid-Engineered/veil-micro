package display

import (
	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/internal/config"
	"github.com/micro-editor/micro/v2/internal/screen"
	"github.com/micro-editor/micro/v2/internal/util"
)

// drawVirtualRows uses the same reveal snapshot and block order as the row model.
// Rows never wrap or hold cursors, and their gutter stays blank.
func (w *BufWindow) drawVirtualRows(line, y int) int {
	if !w.decorationsEnabled() {
		return y
	}
	for _, block := range w.Buf.LineVirtualBlocks(line) {
		if w.decorationSuppressed(line, block.Group) {
			continue
		}
		for _, row := range block.Rows {
			if y >= w.bufHeight {
				return y
			}
			if y >= 0 {
				w.drawVirtualRow(row, y)
			}
			y++
		}
	}
	return y
}

func (w *BufWindow) drawVirtualRow(row buffer.VirtualRow, y int) {
	for x := 0; x < w.gutterOffset+w.bufWidth; x++ {
		screen.SetContent(w.X+x, w.Y+y, ' ', nil, config.DefStyle)
	}
	start := 0
	if !w.Buf.Settings["softwrap"].(bool) {
		start = w.StartCol
	}
	data := []byte(row.Text)
	visualX, col := 0, 0
	tabsize := util.IntOpt(w.Buf.Settings["tabsize"])
	for len(data) > 0 && visualX < start+w.bufWidth {
		r, comb, size := util.DecodeCharacter(data)
		data = data[size:]
		width := displayRuneWidth(r, visualX, tabsize)
		style := config.DefStyle
		for _, span := range row.Spans {
			if col >= span.Start && (span.End == -1 || col < span.End) {
				style = concealStyle(span.Style)
			}
		}
		// A partially clipped wide glyph uses padding, as ordinary text does.
		for cell := 0; cell < width; cell++ {
			x := visualX + cell - start
			if x < 0 || x >= w.bufWidth {
				continue
			}
			ch, combining := ' ', []rune(nil)
			if cell == 0 && r != '\t' && visualX+width <= start+w.bufWidth {
				ch, combining = r, comb
			}
			screen.SetContent(w.X+w.gutterOffset+x, w.Y+y, ch, combining, style)
		}
		visualX += width
		col++
	}
}
