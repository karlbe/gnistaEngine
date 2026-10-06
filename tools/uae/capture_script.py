"""Capture the player's script state from the original game for comparison with the Go
script engine (pkg/game/script).

Starts WinUAE from the in-game savestate, records the initial state of actor slot 0 and the
engine globals, then holds a direction and records every change of the slot's script
position (PC), the actor's frames and the view position.

Usage: py tools/uae/capture_script.py [right|left|up|down|fire|none] [seconds] [pre] > assets-local/uae/trace_right.json

pre is an optional comma-separated list of holds (e.g. right:2.5,left:0.15) that moves the
player before recording; recording starts after the player has come to rest, or at once
if pre ends with "!" (e.g. right:0.5! records pressing down while running).
"""
import json
import os
import struct
import sys
import time

sys.path.insert(0, os.path.dirname(__file__))
import uaectl  # noqa: E402

SLOT, ACTOR_FIELD = 0x2B28, 0x10


def slot_state(u):
    s = u.read(SLOT, 0x1E)
    base = u.hunk_amiga
    pc, actor = struct.unpack(">II", s[0x0C:0x14])
    a = u.amiga_read(actor, 0x20) if actor else bytes(0x20)
    w = lambda b, o: struct.unpack(">h", b[o:o + 2])[0]
    return {
        "x": w(s, 0), "y": w(s, 2),
        "xmask": struct.unpack(">H", s[4:6])[0], "ymask": struct.unpack(">H", s[6:8])[0],
        "pc": pc - base if pc else 0,
        "actor": actor - base if actor else 0,
        "tile_x": s[0x14], "tile_y": s[0x15],
        "sensors": list(s[0x16:0x1C]),
        "upper": {"x": w(a, 0), "y": w(a, 2), "frame": w(a, 4)},
        "lower": {"x": w(a, 0x18), "y": w(a, 0x1A), "frame": w(a, 0x1C)},
    }


def enemies(u):
    """Slots 1-3 as [pc, x, frame +0, frame +$18] and the lift cabin (slot 4) as
    [pc, x, frame, state], or None when free."""
    out = []
    base = u.hunk_amiga
    for k in range(1, 5):
        s = u.read(SLOT + 0x1E * k, 0x1E)
        pc, actor = struct.unpack(">II", s[0x0C:0x14])
        if not actor:
            out.append(None)
            continue
        a = u.amiga_read(actor, 0x20)
        last = a[6:8] if k == 4 else a[0x1C:0x1E]
        out.append([pc - base, struct.unpack(">h", s[0:2])[0], struct.unpack(">h", a[4:6])[0],
                    struct.unpack(">h", last)[0]])
    return out


def enemies_full(u):
    """Slots 1-3 in full (for the start state), or None when free."""
    out = []
    base = u.hunk_amiga
    for k in range(1, 4):
        s = u.read(SLOT + 0x1E * k, 0x1E)
        pc, actor = struct.unpack(">II", s[0x0C:0x14])
        rec = struct.unpack(">I", s[8:12])[0]
        if not actor:
            out.append(None)
            continue
        a = u.amiga_read(actor, 0x20)
        r = u.amiga_read(rec, 0x16)
        h = lambda b, o: struct.unpack(">h", b[o:o + 2])[0]
        out.append({
            "pc": pc - base, "x": h(s, 0), "y": h(s, 2), "xmask": struct.unpack(">H", s[4:6])[0],
            "ymask": struct.unpack(">H", s[6:8])[0], "tile_x": s[0x14], "tile_y": s[0x15],
            "sensors": list(s[0x16:0x1C]), "dead": s[0x1C],
            "scripts": [x - base for x in struct.unpack(">5I", r[:20])], "hp": struct.unpack(">H", r[20:22])[0],
            "upper": {"x": h(a, 0), "y": h(a, 2), "frame": h(a, 4), "state": h(a, 6)},
            "lower": {"x": h(a, 0x18), "y": h(a, 0x1A), "frame": h(a, 0x1C), "state": h(a, 0x1E)},
        })
    return out


def spawner(u):
    """The enemy spawner's state ($508)."""
    return {
        "cells": list(u.read(0xA738, 29 * 26)), "wave": u.word(0xC26), "left": u.word(0xC20),
        "next": (u.long(0xC22) - u.hunk_amiga - 0xBDA) // 2, "delays": list(struct.unpack(">15H", u.read(0xBDA, 30))),
        "toggle": u.read(0xC17, 1)[0] & 1, "random": u.long(0xC2C), "variant": u.long(0xC18) - u.hunk_amiga - 0xC02,
        "off_r": u.word(0xC30), "off_l": u.word(0xC32), "contact": u.word(0xBD8),
    }


def globals_(u):
    w = lambda a: struct.unpack(">h", u.read(a, 2))[0]
    return {
        "dx": w(0x8F5A), "dy": w(0x8F5C), "move_count": u.read(0x8F5F, 1)[0], "moving": u.read(0x8F60, 1)[0],
        "move_wait": u.read(0x8F5E, 1)[0],
        "lift": u.read(0x17C6, 1)[0], "cards": u.read(0x505, 1)[0],
        "climbing": w(0xC34), "blocked": w(0xC1C), "down_flag": w(0x18BC), "fire_latch": w(0x1A24),
        "cell": u.long(0x4EE), "lives": w(0x1C02), "health": u.read(0x1C05, 1)[0],
        "shooter_slot": (u.long(0x1BFC) - u.hunk_amiga - SLOT) // 0x1E if u.long(0x1BFC) else 0,
        "frame_base_upper": w(0xEA0), "frame_base_lower": w(0xEA2), "weapon": w(0x4F4),
        "weapons_owned": u.read(0x504, 1)[0], "charges": w(0x502),
        "view_x": w(0x8F56), "view_y": w(0x8F58),
        "ammo": [list(u.read(0x4F6 + 4 * i, 4)) for i in range(3)],
        "move_bits": u.read(0x506, 1)[0], "loop_117e": u.read(0x117E, 1)[0],
        "resume": u.long(0x17DC) - u.hunk_amiga if u.long(0x17DC) else 0,
        "resume_left": u.long(0x17E0) - u.hunk_amiga if u.long(0x17E0) else 0,
    }


def main():
    direction = sys.argv[1] if len(sys.argv) > 1 else "right"
    seconds = float(sys.argv[2]) if len(sys.argv) > 2 else 4
    pre_arg = sys.argv[3] if len(sys.argv) > 3 else ""
    keep_moving = pre_arg.endswith("!")
    pre = [p.split(":") for p in pre_arg.rstrip("!").split(",")] if pre_arg else []
    u = uaectl.UAE.launch(state=uaectl.STATE)
    try:
        if not u.wait_hunk(30):
            raise RuntimeError("game code not found")
        time.sleep(1)
        u.take_control()
        u.set_input()
        time.sleep(0.5)
        for k, (d, secs) in enumerate(pre):
            parts = d.split("+")
            u.hold([p for p in parts if p in ("up", "down", "left", "right")], fire="fire" in parts,
                   seconds=float(secs))
            if not (keep_moving and k == len(pre) - 1):
                u.set_input()
        if pre and not keep_moving:
            time.sleep(1.5)
        start = {"slot": slot_state(u), "globals": globals_(u), "spawner": spawner(u), "enemies": enemies_full(u)}
        u.set_input([d for d in [direction] if d not in ("none", "fire")], fire=direction == "fire")
        trace, last, t0 = [], None, time.time()
        while time.time() - t0 < seconds:
            # The reads are separate, so the emulator may move on a frame between them; only
            # take samples that read the same twice in a row.
            st, g, e = slot_state(u), globals_(u), enemies(u)
            if (st, g, e) != (slot_state(u), globals_(u), enemies(u)):
                continue
            key = (st["pc"], st["upper"]["frame"], st["lower"]["frame"], g["dx"], g["view_x"], g["view_y"], str(e))
            if key != last:
                trace.append({"t": round(time.time() - t0, 3), "pc": st["pc"], "upper": st["upper"]["frame"],
                              "lower": st["lower"]["frame"], "dx": g["dx"], "view_x": g["view_x"], "view_y": g["view_y"],
                              "x": st["x"], "sensors": st["sensors"], "e": e,
                              "lives": g["lives"], "shooter": g["shooter_slot"], "outcome": struct.unpack(">h", u.read(0x21EA, 2))[0]})
                last = key
            time.sleep(0.002)
        u.set_input()
        u.release_control()
    finally:
        u.kill()
    json.dump({"direction": direction, "seconds": seconds, "pre": sys.argv[3] if len(sys.argv) > 3 else "",
               "start": start, "trace": trace}, sys.stdout, indent=1)


if __name__ == "__main__":
    main()
