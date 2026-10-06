"""Linear disassembly of a hunk-relative address range of ns (hex): py tools/nsdis.py START END"""
import sys
from capstone import CS_ARCH_M68K, CS_MODE_M68K_000, Cs

code = open("assets-local/adf/ns", "rb").read()[36:]
md = Cs(CS_ARCH_M68K, CS_MODE_M68K_000)
a, end = int(sys.argv[1], 16), int(sys.argv[2], 16)
while a < end:
    i = next(md.disasm(code[a:a + 10], a, 1), None)
    if i is None:
        print("%04x dc.w $%04x" % (a, int.from_bytes(code[a:a + 2], "big")))
        a += 2
        continue
    print("%04x %-8s %s" % (i.address, i.mnemonic, i.op_str))
    a += i.size
