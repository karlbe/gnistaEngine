"""List every instruction in ns that uses an absolute address: py tools/nsxref.py ADDR...

Addresses are hunk-relative hex (e.g. 1c05 bd8). Finds absolute long operands in the code
hunk, so data references through registers are not listed.
"""
import sys

from capstone import CS_ARCH_M68K, CS_MODE_M68K_000, Cs

code = open("assets-local/adf/ns", "rb").read()[36:36 + 47816]
md = Cs(CS_ARCH_M68K, CS_MODE_M68K_000)

for arg in sys.argv[1:]:
    target = int(arg, 16)
    needle = target.to_bytes(4, "big")
    a = code.find(needle)
    while a >= 0:
        for back in (2, 4, 6, 8):
            s = a - back
            ins = next(md.disasm(code[s:s + 14], s, 1), None)
            if ins and s + ins.size > a and f"${target:x}" in ins.op_str:
                print(f"{arg:>6}  {ins.address:04x}  {ins.mnemonic} {ins.op_str}")
                break
        a = code.find(needle, a + 2)
