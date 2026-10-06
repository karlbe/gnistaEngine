"""Render the level: a block-id matrix stored in the ns code hunk ($A333, 29 columns)
where each id selects a 20x4-tile block (80 bytes) of RoomData, tiles from NSIIcons.

Usage: py tools/levelmap2png.py <adf_dir> <palette_png> <out.png> [scale_divisor]
"""
import os
import struct
import sys

sys.path.insert(0, os.path.dirname(__file__))
from jbob2png import entries, sprite, write_rgba
from pngread import read_png

MAP_ADDR, STRIDE, BW, BH = 0xA333, 29, 20, 4

if __name__ == "__main__":
    adf, pal_png, out = sys.argv[1:4]
    div = int(sys.argv[4]) if len(sys.argv) > 4 else 1
    ns = open(os.path.join(adf, "ns"), "rb").read()
    code = ns[36:36 + 47816]
    rows_n = (len(code) - MAP_ADDR) // STRIDE
    grid = [code[MAP_ADDR + y * STRIDE:MAP_ADDR + (y + 1) * STRIDE] for y in range(rows_n)]
    used = [(x, y) for y, r in enumerate(grid) for x, v in enumerate(r) if v]
    x0, x1 = min(x for x, _ in used), max(x for x, _ in used)
    y0, y1 = min(y for _, y in used), max(y for _, y in used)
    print(f"map {STRIDE}x{rows_n}, non-zero bbox x {x0}-{x1}, y {y0}-{y1}")
    pal = read_png(pal_png)[2]
    es, body = entries(open(os.path.join(adf, "NSIIcons"), "rb").read())
    tiles = {e["i"]: sprite(e, body) for e in es if e["valid"] and e["w"] == 16 and e["h"] == 16}
    rd = open(os.path.join(adf, "RoomData"), "rb").read()
    W, H = (x1 - x0 + 1) * BW * 16, (y1 - y0 + 1) * BH * 16
    rows = [bytearray(W * 4) for _ in range(H)]
    for by in range(y0, y1 + 1):
        for bx in range(x0, x1 + 1):
            bid = grid[by][bx]
            for ty in range(BH):
                for tx in range(BW):
                    t = tiles.get(rd[bid * 80 + ty * BW + tx])
                    px0, py0 = ((bx - x0) * BW + tx) * 16, ((by - y0) * BH + ty) * 16
                    for j in range(256):
                        v, m = t[j] if t else (0, 0)
                        o = (px0 + j % 16) * 4
                        rows[py0 + j // 16][o:o + 4] = bytes(pal[v]) + b"\xff" if t and m else b"\0\0\0\xff"
    if div > 1:
        rows = [bytearray(b"".join(bytes(r[x * 4:x * 4 + 4]) for x in range(0, W, div))) for r in rows[::div]]
        W, H = W // div, len(rows)
    write_rgba(out, W, H, rows)
    print("wrote", out, W, H)
