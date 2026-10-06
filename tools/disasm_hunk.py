"""Linear 68000 disassembly of the first CODE hunk of an AmigaOS hunk executable.

Usage: py tools/disasm_hunk.py <exe> <out.asm>
Prints a summary of custom-chip register and library-call usage.
"""
import re
import struct
import sys
from collections import Counter

from capstone import CS_ARCH_M68K, CS_MODE_M68K_000, Cs

b = open(sys.argv[1], "rb").read()
u = lambda i: struct.unpack(">I", b[i:i + 4])[0]
i = 8
nh = u(i)
i += 12 + 4 * nh
assert u(i) & 0x3fffffff == 0x3e9
n = u(i + 4) * 4
code = b[i + 8:i + 8 + n]
i += 8 + n
relocs = []
if u(i) & 0x3fffffff == 0x3ec:
    i += 4
    while u(i):
        cnt = u(i)
        i += 8
        relocs += [u(i + 4 * k) for k in range(cnt)]
        i += 4 * cnt
reloc_set = set(relocs)

md = Cs(CS_ARCH_M68K, CS_MODE_M68K_000)
md.detail = False
out, off, bad = [], 0, 0
mn = Counter()
while off < len(code):
    ins = next(md.disasm(code[off:off + 10], off, 1), None)
    if ins is None:
        out.append(f"{off:06x}: dc.w ${code[off]:02x}{code[off+1]:02x}")
        off += 2
        bad += 1
        continue
    mn[ins.mnemonic] += 1
    out.append(f"{off:06x}: {ins.mnemonic} {ins.op_str}")
    off += ins.size
open(sys.argv[2], "w").write("\n".join(out))

txt = "\n".join(out)
print(f"code {len(code)} bytes, {len(out)} lines, {bad} undecodable, {len(relocs)} relocs")
print("top mnemonics:", mn.most_common(12))
custom = Counter(m for m in re.findall(r"\$(dff[0-9a-f]{3})", txt))
cia = Counter(m for m in re.findall(r"\$(bf[de][0-9a-f]{3})", txt))
print("custom-chip regs:", sorted(custom.items())[:40])
print("CIA regs:", sorted(cia.items())[:10])
lib = Counter(re.findall(r"jsr\s+\$?-?(\d+)\(a6\)|jsr\s+(-?\d+)\(a6\)", txt))
print("jsr -n(a6):", Counter(re.findall(r"jsr\s+\(?(-\d+)\s*\(a6\)|jsr\s+-\$?[0-9a-f]+\(a6\)", txt)).most_common(10))
print("exec base ref (move.l $4):", len(re.findall(r"\$4\.l|\(\$4\)", txt)))
