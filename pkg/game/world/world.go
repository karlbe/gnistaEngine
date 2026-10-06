// Package world holds the level geometry: a matrix of block ids, where each block is a
// fixed-size group of tiles (see docs/adf-inventory.md, "kartmatrisen i koden").
package world

import "fmt"

// Block geometry in tiles ($9112, $9114 in the original), tile size in pixels.
const (
	BlockW   = 20
	BlockH   = 4
	TileSize = 16
)

// Level is the block-id matrix plus the block library it refers to.
type Level struct {
	Cols, Rows int
	Cells      []uint8 // block id per cell, row-major, Cols per row
	Blocks     []uint8 // BlockW*BlockH tile ids per block
	T          Tables  // enemy triggers, rooms, lift cards and the start
}

func NewLevel(cols, rows int, cells, blocks []uint8) (*Level, error) {
	if cols <= 0 || rows <= 0 || len(cells) < cols*rows {
		return nil, fmt.Errorf("level matrix %dx%d needs %d bytes, have %d", cols, rows, cols*rows, len(cells))
	}
	if len(blocks)%(BlockW*BlockH) != 0 {
		return nil, fmt.Errorf("block data size %d is not a multiple of %d", len(blocks), BlockW*BlockH)
	}
	return &Level{Cols: cols, Rows: rows, Cells: cells[:cols*rows], Blocks: blocks}, nil
}

// WidthTiles and HeightTiles give the level size in tiles.
func (l *Level) WidthTiles() int  { return l.Cols * BlockW }
func (l *Level) HeightTiles() int { return l.Rows * BlockH }

// Tile returns the tile id at tile coordinates (tx, ty), or -1 outside the level.
func (l *Level) Tile(tx, ty int) int {
	if tx < 0 || ty < 0 || tx >= l.WidthTiles() || ty >= l.HeightTiles() {
		return -1
	}
	id := int(l.Cells[ty/BlockH*l.Cols+tx/BlockW])
	o := id*BlockW*BlockH + ty%BlockH*BlockW + tx%BlockW
	if o >= len(l.Blocks) {
		return -1
	}
	return int(l.Blocks[o])
}
