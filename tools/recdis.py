"""Recursive-descent 68000 disassembler for the first CODE hunk of an Amiga hunk exe.

Follows branches/calls from address 0 (certain code), then from relocated
pointers that land on plausible code (candidates). Everything else is data.

Usage: py tools/recdis.py <exe> <out.asm>
"""
import re
import struct
import sys

from capstone import CS_ARCH_M68K, CS_MODE_M68K_000, Cs

b = open(sys.argv[1], "rb").read()
u = lambda i: struct.unpack(">I", b[i:i + 4])[0]
i = 8
nh = u(i)
i += 12 + 4 * nh
n = u(i + 4) * 4
code = b[i + 8:i + 8 + n]
i += 8 + n
reloc_sites = []
if u(i) & 0x3fffffff == 0x3ec:
    i += 4
    while u(i):
        cnt = u(i)
        i += 8
        reloc_sites += [u(i + 4 * k) for k in range(cnt)]
        i += 4 * cnt
reloc_vals = {s: struct.unpack(">I", code[s:s + 4])[0] for s in reloc_sites if s + 4 <= len(code)}

md = Cs(CS_ARCH_M68K, CS_MODE_M68K_000)
TARGET = re.compile(r"\$([0-9a-f]+)(?:\.l)?$")
insns = {}      # addr -> (size, text)
labels = set()
unresolved = []


def target(op):
    m = TARGET.search(op.strip().split(",")[-1].strip())
    return int(m.group(1), 16) if m else None


def trace(seed, tag):
    work, added = [seed], 0
    while work:
        a = work.pop()
        while 0 <= a < len(code) and a % 2 == 0 and a not in insns:
            ins = next(md.disasm(code[a:a + 10], a, 1), None)
            if ins is None:
                break
            mn, op = ins.mnemonic, ins.op_str
            insns[a] = (ins.size, f"{mn} {op}".strip(), tag)
            added += 1
            nxt = a + ins.size
            t = target(op)
            if mn in ("rts", "rte", "rtr", "illegal", "stop"):
                break
            if mn in ("jmp", "bra", "bra.w", "bra.b", "bra.s") or mn.startswith("bra"):
                if t is not None and "(" not in op:
                    labels.add(t)
                    work.append(t)
                else:
                    unresolved.append((a, mn, op))
                break
            if mn.startswith("b") or mn.startswith("db") or mn in ("jsr",):
                if t is not None and "(" not in op.split(",")[-1]:
                    labels.add(t)
                    work.append(t)
                elif mn == "jsr":
                    unresolved.append((a, mn, op))
            a = nxt
    return added


trace(0, "C")
certain = len(insns)
# candidates: relocated pointers into code, tried in address order
for v in sorted(set(reloc_vals.values())):
    if v < len(code) and v % 2 == 0 and v not in insns:
        snapshot = dict(insns)
        trace(v, "?")
cand = len(insns) - certain

covered = bytearray(len(code))
for a, (sz, _, _) in insns.items():
    for k in range(sz):
        covered[a + k] = 1
out = []
a = 0
while a < len(code):
    if a in insns:
        sz, txt, tag = insns[a]
        out.append((f"L{a:06x}:" if a in labels else "         ") + f" {a:06x} {tag} {txt}")
        a += sz
    else:
        j = a
        while j < len(code) and j not in insns:
            j += 1
        row = code[a:j]
        out.append(f"          {a:06x} D dc.b {len(row)} bytes" + (" (zeros)" if not any(row) else ""))
        a = j
open(sys.argv[2], "w").write("\n".join(out))
print(f"code {len(code)} B; certain insns {certain}, candidate insns {cand}")
print(f"covered {sum(covered)} B ({sum(covered)*100//len(code)}%), labels {len(labels)}, unresolved indirect {len(unresolved)}")
print("unresolved:", [f"{a:x} {m} {o}" for a, m, o in unresolved[:12]])
