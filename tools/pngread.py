"""Minimal PNG reader for 8-bit indexed/RGB(A) non-interlaced images. Returns (w, h, palette, rows)."""
import struct
import zlib


def read_png(path):
    d = open(path, "rb").read()
    i, idat, pal = 8, b"", None
    while i < len(d):
        n = struct.unpack(">I", d[i:i + 4])[0]
        t = d[i + 4:i + 8]
        c = d[i + 8:i + 8 + n]
        if t == b"IHDR":
            w, h, bd, ct = struct.unpack(">IIBB", c[:10])
        elif t == b"PLTE":
            pal = [tuple(c[k:k + 3]) for k in range(0, n, 3)]
        elif t == b"IDAT":
            idat += c
        i += 12 + n
    bpp = {3: 1, 2: 3, 6: 4, 0: 1}[ct]
    raw = zlib.decompress(idat)
    stride = w * bpp
    rows, prev = [], bytearray(stride)
    for y in range(h):
        f = raw[y * (stride + 1)]
        cur = bytearray(raw[y * (stride + 1) + 1:(y + 1) * (stride + 1)])
        for x in range(stride):
            a = cur[x - bpp] if x >= bpp else 0
            b = prev[x]
            c2 = prev[x - bpp] if x >= bpp else 0
            if f == 1: cur[x] = (cur[x] + a) & 255
            elif f == 2: cur[x] = (cur[x] + b) & 255
            elif f == 3: cur[x] = (cur[x] + (a + b) // 2) & 255
            elif f == 4:
                p = a + b - c2
                pa, pb, pc = abs(p - a), abs(p - b), abs(p - c2)
                cur[x] = (cur[x] + (a if pa <= pb and pa <= pc else b if pb <= pc else c2)) & 255
        rows.append(bytes(cur))
        prev = cur
    return w, h, pal, rows, ct
