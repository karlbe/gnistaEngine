"""Convert IFF ILBM files (ByteRun1, up to 8 planes) to PNG, no dependencies.

Usage: python tools/ilbm2png.py <out_dir> <file.ilbm>...
"""
import os
import struct
import sys
import zlib


def chunks(b):
    i = 12
    while i + 8 <= len(b):
        n = struct.unpack(">I", b[i + 4:i + 8])[0]
        yield b[i:i + 4].decode("latin-1"), b[i + 8:i + 8 + n]
        i += 8 + n + (n & 1)


def unpack_rle(d, size):
    out = bytearray()
    i = 0
    while len(out) < size and i < len(d):
        c = d[i]
        i += 1
        if c < 128:
            out += d[i:i + c + 1]
            i += c + 1
        elif c > 128:
            out += d[i:i + 1] * (257 - c)
            i += 1
    return bytes(out)


def decode(path):
    b = open(path, "rb").read()
    palette, body, bm = None, None, None
    for cid, d in chunks(b):
        if cid == "BMHD":
            bm = struct.unpack(">HHhhBBBBHBBhh", d[:20])
        elif cid == "CMAP":
            palette = [tuple(d[i:i + 3]) for i in range(0, len(d) - 2, 3)]
        elif cid == "BODY":
            body = d
    w, h, _, _, planes, mask, comp = bm[:7]
    rb = (w + 15) // 16 * 2
    nplanes = planes + (1 if mask == 1 else 0)
    raw = unpack_rle(body, rb * nplanes * h) if comp else body
    pix = bytearray()
    for y in range(h):
        row = raw[y * rb * nplanes:(y + 1) * rb * nplanes]
        for x in range(w):
            v = 0
            for p in range(planes):
                v |= ((row[p * rb + (x >> 3)] >> (7 - (x & 7))) & 1) << p
            pix.append(v)
    return w, h, palette, bytes(pix)


def write_png(path, w, h, palette, pix):
    def chunk(t, d):
        c = struct.pack(">I", len(d)) + t + d
        return c + struct.pack(">I", zlib.crc32(t + d))
    raw = b"".join(b"\0" + pix[y * w:(y + 1) * w] for y in range(h))
    plte = b"".join(bytes(c) for c in palette)
    open(path, "wb").write(
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 3, 0, 0, 0))
        + chunk(b"PLTE", plte) + chunk(b"IDAT", zlib.compress(raw, 9))
        + chunk(b"IEND", b""))


if __name__ == "__main__":
    out = sys.argv[1]
    os.makedirs(out, exist_ok=True)
    for f in sys.argv[2:]:
        w, h, pal, pix = decode(f)
        write_png(os.path.join(out, os.path.basename(f) + ".png"), w, h, pal, pix)
        print(os.path.basename(f), w, h)
