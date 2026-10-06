"""Decode a FORM JBOB sprite bank into an RGBA atlas PNG plus a JSON index.

Entry layout in BHDR (24 bytes, big endian), inferred from the data:
  u32 body offset, u32 mask-plane offset, u32 plane size, u16 flags,
  u16 (rowbytes + 2), u16 height, u16 width in words, 4-byte tag.
Planes are stored consecutively, colour planes first, the mask plane last.

Usage: python tools/jbob2png.py <bank> <palette.ilbm> <out_prefix>
"""
import json
import struct
import sys
import zlib

sys.path.insert(0, __file__.rsplit("tools", 1)[0] + "tools")
from ilbm2png import decode


def entries(d):
    i, ch = 12, {}
    while i + 8 <= len(d):
        n = struct.unpack(">I", d[i + 4:i + 8])[0]
        ch[d[i:i + 4]] = d[i + 8:i + 8 + n]
        i += 8 + n + (n & 1)
    hdr, body = ch[b"BHDR"], ch[b"BODY"]
    out = []
    for k in range(len(hdr) // 24):
        off, moff, psize, flags, b2, h, ww, tag = struct.unpack(">IIIHHHH4s", hdr[k * 24:k * 24 + 24])
        rb = ww * 2
        valid = (rb * h == psize and moff >= off and (moff - off) % psize == 0
                 and moff + psize <= len(body) and b2 == rb + 2)
        out.append(dict(i=k, off=off, moff=moff, psize=psize, flags=flags, h=h,
                        w=ww * 16, planes=(moff - off) // psize if psize else 0,
                        tag=tag.decode("latin-1"), valid=valid))
    return out, body


def sprite(e, body):
    w, h, rb = e["w"], e["h"], e["w"] // 8
    planes = [body[e["off"] + p * e["psize"]:e["off"] + (p + 1) * e["psize"]] for p in range(e["planes"])]
    mask = body[e["moff"]:e["moff"] + e["psize"]]
    px = []
    for y in range(h):
        for x in range(w):
            o, bit = y * rb + (x >> 3), 7 - (x & 7)
            v = sum(((pl[o] >> bit) & 1) << p for p, pl in enumerate(planes))
            px.append((v, (mask[o] >> bit) & 1))
    return px


def write_rgba(path, w, h, rows):
    def chunk(t, d):
        return struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t + d))
    raw = b"".join(b"\0" + bytes(r) for r in rows)
    open(path, "wb").write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 6, 0, 0, 0))
                           + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b""))


if __name__ == "__main__":
    bank, pal_src, prefix = sys.argv[1:4]
    d = open(bank, "rb").read()
    es, body = entries(d)
    _, _, pal, _ = decode(pal_src)
    good = [e for e in es if e["valid"]]
    print(f"{len(es)} entries, {len(good)} valid")
    AW = 640
    x = y = rowh = 0
    placed = []
    for e in good:
        if x + e["w"] > AW:
            x, y, rowh = 0, y + rowh + 1, 0
        placed.append((e, x, y))
        x += e["w"] + 1
        rowh = max(rowh, e["h"])
    AH = y + rowh + 1
    rows = [bytearray(AW * 4) for _ in range(AH)]
    for e, ox, oy in placed:
        px = sprite(e, body)
        for j, (v, m) in enumerate(px):
            yy, xx = oy + j // e["w"], ox + j % e["w"]
            r, g, b = pal[v] if v < len(pal) else (255, 0, 255)
            rows[yy][xx * 4:xx * 4 + 4] = bytes((r, g, b, 255 if m else 0))
        e["atlas"] = [ox, oy]
    write_rgba(prefix + ".png", AW, AH, rows)
    json.dump(es, open(prefix + ".json", "w"), indent=1)
    from collections import Counter
    print(Counter((e["planes"], e["w"], e["h"]) for e in good).most_common(8))
    print("invalid:", [e["i"] for e in es if not e["valid"]][:20])
