"""Analyse the game's bytecode script engine in ns and disassemble its scripts.

The interpreter at $C7E reads an opcode byte from (a5)+, looks up a handler in the long
table at $21EC and jumps to it; handlers read operands from (a5)+ and return to $C7E.

Usage:
  py tools/scriptdis.py ops                 list opcodes, handlers and operand counts
  py tools/scriptdis.py scripts [out.txt]   disassemble every script in the table at $2BBE
"""
import re
import struct
import sys

from capstone import CS_ARCH_M68K, CS_MODE_M68K_000, Cs

NS = "assets-local/adf/ns"
DISPATCH, OPTABLE, SCRIPTTABLE = 0xC7E, 0x21EC, 0x2BBE
GOTO_TAIL = 0x10F8  # op34's body: reads the script number and jumps

code = open(NS, "rb").read()[36:36 + 47816]
md = Cs(CS_ARCH_M68K, CS_MODE_M68K_000)
L = lambda a: struct.unpack(">I", code[a:a + 4])[0]


def handlers():
    out = []
    a = OPTABLE
    while a + 4 <= len(code):
        h = L(a)
        if h >= len(code) or h % 2 or h < 0x100:
            break
        out.append(h)
        a += 4
    return out


def analyse(h, limit=80):
    """Follow a handler linearly (and through forward branches) until it returns to the
    dispatcher. Returns (operand bytes read on the longest path, ends, notes)."""
    reads, notes, ends = 0, [], "?"
    addr = h
    for _ in range(limit):
        ins = next(md.disasm(code[addr:addr + 10], addr, 1), None)
        if ins is None:
            ends = "bad"
            break
        op = ins.op_str
        reads += op.count("(a5)+") * (2 if ins.mnemonic.endswith(".w") and "(a5)+," in op else 1)
        # Handlers that inspect operands via a0 skip them with adda/addq #n,a5.
        m = re.fullmatch(r"#\$([0-9a-f]+), a5", op)
        if m and ins.mnemonic.split(".")[0] in ("adda", "addq"):
            reads += int(m.group(1), 16)
        if "(a5)" in op and "(a5)+" not in op:
            notes.append(f"{ins.mnemonic} {op}")
        if ins.mnemonic.startswith("bra") and op.endswith(f"${DISPATCH:x}"):
            ends = "next"
            break
        if ins.mnemonic in ("rts", "jmp") or ins.mnemonic.startswith("bra"):
            ends = f"{ins.mnemonic} {op}"
            if op.endswith(f"${GOTO_TAIL:x}"):
                reads += 1  # the goto tail reads the script number
            break
        addr += ins.size
    return reads, ends, notes


def op_table():
    return [(i, h) + analyse(h) for i, h in enumerate(handlers())]


def scripts():
    ptrs = []
    a = SCRIPTTABLE
    while a + 4 <= len(code):
        p = L(a)
        if p >= len(code) or p < 0x2000:
            break
        ptrs.append(p)
        a += 4
    return ptrs


def dis_script(start, ops, end_hint):
    lines, a = [], start
    while a < end_hint and a < len(code):
        op = code[a]
        if op >= len(ops):
            lines.append(f"  {a:06x}: db ${op:02x}   ; not an opcode")
            a += 1
            continue
        _, h, n, ends, _ = ops[op]
        args = list(code[a + 1:a + 1 + n])
        lines.append(f"  {a:06x}: op{op:02d} " + " ".join(f"{x:3d}" for x in args) + ("" if ends == "next" else f"   ; -> {ends}"))
        a += 1 + n
        if ends.endswith(("$18be", f"${GOTO_TAIL:x}")):
            continue  # conditional goto / op37 loop: falls through when not taken
        if ends.startswith(("bra", "jmp", "bad", "?")):  # operand count unknown past here
            break
    return lines


if __name__ == "__main__":
    cmd = sys.argv[1] if len(sys.argv) > 1 else ""
    ops = op_table()
    if cmd == "ops":
        for i, h, n, ends, notes in ops:
            print(f"op{i:02d} handler ${h:04x} operands {n} ends {ends}" + (f"  notes {notes}" if notes else ""))
    elif cmd == "scripts":
        ptrs = scripts()
        out = []
        order = sorted(set(ptrs))
        for k, p in enumerate(ptrs):
            nxt = min([q for q in order if q > p] or [p + 256])
            out.append(f"script {k} @ ${p:04x}")
            out += dis_script(p, ops, nxt)
        text = "\n".join(out)
        if len(sys.argv) > 2:
            open(sys.argv[2], "w").write(text)
            print(f"{len(ptrs)} scripts -> {sys.argv[2]}")
        else:
            print(text)
    else:
        print(__doc__)
