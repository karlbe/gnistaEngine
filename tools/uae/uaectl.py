"""Drive WinUAE for automated tests of the original game (Windows only, ctypes).

- launch WinUAE with a config (optionally from a savestate), auto-dismiss message boxes
- attach to an already running WinUAE (e.g. while someone plays)
- capture the emulator window to PNG, send key presses (joystick = cursor keys + right ctrl)
- locate the running `ns` code hunk in emulator memory and read/write game variables by
  their hunk-relative addresses (as used in docs/re)

CLI:
  py tools/uae/uaectl.py boot              boot to gameplay (pressing fire) and save ingame.uss
  py tools/uae/uaectl.py launch [--fresh]  start from ingame.uss (or a fresh boot) and leave it running
  py tools/uae/uaectl.py watch [ADDR...]   attach to a running WinUAE and print variables every 0.5 s
  py tools/uae/uaectl.py shot FILE.png     attach and take a screenshot
"""
import ctypes
import ctypes.wintypes as wt
import os
import struct
import subprocess
import sys
import time
from collections import Counter

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from jbob2png import write_rgba  # noqa: E402

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
WINUAE = os.environ.get("WINUAE", r"E:\Spel\Amiga\WinUAE\winuae64.exe")
UAEDIR = os.path.join(ROOT, "assets-local", "uae")
CONFIG = os.path.join(UAEDIR, "pgi.uae")
STATE = os.path.join(UAEDIR, "ingame.uss")
NS = os.path.join(ROOT, "assets-local", "adf", "ns")

# Variables shown by `watch` when no addresses are given: (address, size, name)
DEFAULT_VARS = [(0x21EA, 2, "outcome"), (0x8F56, 2, "view_x"), (0x8F58, 2, "view_y"),
                (0x8F72, 2, "blk_col"), (0x8F74, 2, "blk_row")]

user32 = ctypes.WinDLL("user32", use_last_error=True)
gdi32 = ctypes.WinDLL("gdi32", use_last_error=True)
kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)

user32.SetProcessDPIAware()
kernel32.OpenProcess.restype = wt.HANDLE
kernel32.ReadProcessMemory.argtypes = [wt.HANDLE, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_size_t, ctypes.POINTER(ctypes.c_size_t)]
kernel32.WriteProcessMemory.argtypes = [wt.HANDLE, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_size_t, ctypes.POINTER(ctypes.c_size_t)]
kernel32.VirtualQueryEx.argtypes = [wt.HANDLE, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_size_t]
kernel32.VirtualQueryEx.restype = ctypes.c_size_t
user32.PostMessageW.argtypes = [wt.HWND, wt.UINT, wt.WPARAM, wt.LPARAM]

MAILBOX_JOY, MAILBOX_FIRE = 0x3F0, 0x3F4  # chip RAM, unused 68000 user vectors

PROCESS_ALL = 0x0438  # VM_READ | VM_WRITE | VM_OPERATION | QUERY_INFORMATION
BM_CLICK, WM_CLOSE = 0x00F5, 0x0010
KEYEVENTF_EXTENDEDKEY, KEYEVENTF_KEYUP, KEYEVENTF_SCANCODE = 0x1, 0x2, 0x8

# (scancode, extended)
KEYS = {
    "up": (0x48, True), "down": (0x50, True), "left": (0x4B, True), "right": (0x4D, True),
    "fire": (0x1D, True), "space": (0x39, False), "return": (0x1C, False), "esc": (0x01, False),
    "f1": (0x3B, False), "f2": (0x3C, False), "f3": (0x3D, False), "f4": (0x3E, False),
}


class MBI(ctypes.Structure):
    _fields_ = [("BaseAddress", ctypes.c_void_p), ("AllocationBase", ctypes.c_void_p),
                ("AllocationProtect", wt.DWORD), ("PartitionId", wt.WORD), ("RegionSize", ctypes.c_size_t),
                ("State", wt.DWORD), ("Protect", wt.DWORD), ("Type", wt.DWORD)]


class KEYBDINPUT(ctypes.Structure):
    _fields_ = [("wVk", wt.WORD), ("wScan", wt.WORD), ("dwFlags", wt.DWORD), ("time", wt.DWORD),
                ("dwExtraInfo", ctypes.c_size_t)]


class INPUT(ctypes.Structure):
    class _U(ctypes.Union):
        _fields_ = [("ki", KEYBDINPUT), ("pad", ctypes.c_byte * 32)]
    _fields_ = [("type", wt.DWORD), ("u", _U)]


def _ns_code_and_relocs():
    b = open(NS, "rb").read()
    u = lambda i: struct.unpack(">I", b[i:i + 4])[0]
    i = 8
    i += 12 + 4 * u(i)
    n = u(i + 4) * 4
    code = b[i + 8:i + 8 + n]
    i += 8 + n
    relocs = []
    if u(i) & 0x3FFFFFFF == 0x3EC:
        i += 4
        while u(i):
            cnt = u(i)
            relocs += [u(i + 8 + 4 * k) for k in range(cnt)]
            i += 8 + 4 * cnt
    return code, relocs


def _windows_of(pid):
    found = []

    @ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)
    def cb(hwnd, _):
        p = wt.DWORD()
        user32.GetWindowThreadProcessId(hwnd, ctypes.byref(p))
        if p.value == pid and user32.IsWindowVisible(hwnd):
            buf = ctypes.create_unicode_buffer(256)
            user32.GetWindowTextW(hwnd, buf, 256)
            found.append((hwnd, buf.value))
        return True
    user32.EnumWindows(cb, 0)
    return found


def _running_pid():
    out = subprocess.run(["tasklist", "/FI", "IMAGENAME eq winuae64.exe", "/FO", "CSV", "/NH"],
                         capture_output=True, text=True).stdout
    for line in out.splitlines():
        parts = line.strip('"').split('","')
        if len(parts) > 1 and parts[0].lower() == "winuae64.exe":
            return int(parts[1])
    return None


class UAE:
    def __init__(self, pid=None, proc=None):
        self.proc = proc
        self.pid = pid or proc.pid
        self.h = kernel32.OpenProcess(PROCESS_ALL, False, self.pid)
        self.hunk_host = None
        self.hunk_amiga = None

    @classmethod
    def launch(cls, state=None, extra=None):
        args = [WINUAE, "-f", CONFIG]
        extra = dict(extra or {})
        if state:
            extra["statefile"] = state  # the -statefile flag is ignored together with -f
        for k, v in extra.items():
            args += ["-s", f"{k}={v}"]
        si = subprocess.STARTUPINFO()
        si.dwFlags |= subprocess.STARTF_USESHOWWINDOW
        si.wShowWindow = 4  # SW_SHOWNOACTIVATE
        prev = user32.GetForegroundWindow()
        uae = cls(proc=subprocess.Popen(args, startupinfo=si))
        # WinUAE activates itself anyway; hand focus back to whatever the user was using.
        t0 = time.time()
        while time.time() - t0 < 10 and not uae.main_window():
            uae.dismiss_dialogs()
            time.sleep(0.2)
        time.sleep(0.5)
        if prev:
            user32.SetForegroundWindow(prev)
        return uae

    @classmethod
    def attach(cls):
        pid = _running_pid()
        if not pid:
            raise RuntimeError("WinUAE is not running")
        return cls(pid=pid)

    # --- windows ---------------------------------------------------------
    def dismiss_dialogs(self):
        """Click OK on WinUAE message boxes (e.g. the harmless 'ROM key file' warning)."""
        n = 0
        for hwnd, title in _windows_of(self.pid):
            if "message" not in title.lower():
                continue

            @ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)
            def click(child, _):
                cls = ctypes.create_unicode_buffer(64)
                user32.GetClassNameW(child, cls, 64)
                if cls.value == "Button":
                    user32.SendMessageW(child, BM_CLICK, 0, 0)
                return True
            user32.EnumChildWindows(hwnd, click, 0)
            n += 1
        return n

    def main_window(self):
        # The title changes (capture state etc.), so pick the process's non-dialog window.
        for hwnd, _ in _windows_of(self.pid):
            cls = ctypes.create_unicode_buffer(64)
            user32.GetClassNameW(hwnd, cls, 64)
            if cls.value != "#32770":
                return hwnd
        return None

    def capture(self):
        """Return (w, h, rows of RGBA bytes) of the emulator window's client area."""
        hwnd = self.main_window()
        if not hwnd:
            raise RuntimeError("no emulator window")
        rc = wt.RECT()
        user32.GetClientRect(hwnd, ctypes.byref(rc))
        w, h = rc.right, rc.bottom
        wdc = user32.GetDC(hwnd)
        mdc = gdi32.CreateCompatibleDC(wdc)
        bmp = gdi32.CreateCompatibleBitmap(wdc, w, h)
        gdi32.SelectObject(mdc, bmp)
        user32.PrintWindow(hwnd, mdc, 3)  # PW_CLIENTONLY | PW_RENDERFULLCONTENT
        bih = struct.pack("<IiiHHIIiiII", 40, w, -h, 1, 32, 0, 0, 0, 0, 0, 0)
        buf = ctypes.create_string_buffer(w * h * 4)
        gdi32.GetDIBits(mdc, bmp, 0, h, buf, ctypes.create_string_buffer(bih), 0)
        gdi32.DeleteObject(bmp)
        gdi32.DeleteDC(mdc)
        user32.ReleaseDC(hwnd, wdc)
        raw = buf.raw
        rows = []
        for y in range(h):
            r = bytearray(raw[y * w * 4:(y + 1) * w * 4])
            r[0::4], r[2::4] = r[2::4], r[0::4]  # BGRA -> RGBA
            r[3::4] = b"\xff" * w
            rows.append(r)
        return w, h, rows

    def screenshot(self, path):
        w, h, rows = self.capture()
        write_rgba(path, w, h, rows)
        return w, h

    def in_game(self):
        """Gameplay has started: the joystick routine ($235C) writes a non-zero direction mask
        to $23CA every frame (bit 0 = no direction); the value in the file is 0."""
        return self.read(0x23CA, 1) != b"\x00"

    def key(self, name, hold=0.08):
        sc, ext = KEYS[name]
        flags = KEYEVENTF_SCANCODE | (KEYEVENTF_EXTENDEDKEY if ext else 0)
        for f in (flags, flags | KEYEVENTF_KEYUP):
            inp = INPUT(type=1)
            inp.u.ki = KEYBDINPUT(0, sc, f, 0, 0)
            user32.SendInput(1, ctypes.byref(inp), ctypes.sizeof(INPUT))
            time.sleep(hold)

    # --- memory -----------------------------------------------------------
    def _read_host(self, addr, n):
        buf = ctypes.create_string_buffer(n)
        got = ctypes.c_size_t()
        if not kernel32.ReadProcessMemory(self.h, ctypes.c_void_p(addr), buf, n, ctypes.byref(got)):
            raise OSError(ctypes.get_last_error())
        return buf.raw[:got.value]

    def _regions(self):
        addr, mbi = 0, MBI()
        while kernel32.VirtualQueryEx(self.h, ctypes.c_void_p(addr), ctypes.byref(mbi), ctypes.sizeof(mbi)):
            if mbi.State == 0x1000 and mbi.Protect in (0x04, 0x40) and mbi.RegionSize < 1 << 31:
                yield mbi.BaseAddress or 0, mbi.RegionSize
            addr = (mbi.BaseAddress or 0) + mbi.RegionSize

    def find_hunk(self):
        """Find the running (relocated) ns code hunk in emulator memory. Returns True when found.

        Unrelocated copies (e.g. DOS read buffers) are rejected: most sampled reloc sites
        must differ from the file by the same non-zero load base (some sites are pointer
        variables the game overwrites at runtime)."""
        code, relocs = _ns_code_and_relocs()
        anchor = code.find(b"df0:RoomData")
        strings = slice(0x7B6C, 0x7BB4)  # filename table, never written at runtime
        sites = relocs[::max(1, len(relocs) // 64)]
        for base, size in self._regions():
            try:
                data = self._read_host(base, size)
            except OSError:
                continue
            i = data.find(b"df0:RoomData")
            while i >= 0:
                start = i - anchor
                if start >= 0 and start + len(code) <= len(data) \
                        and data[start + strings.start:start + strings.stop] == code[strings]:
                    votes = Counter((struct.unpack(">I", data[start + s:start + s + 4])[0]
                                     - struct.unpack(">I", code[s:s + 4])[0]) & 0xFFFFFFFF for s in sites)
                    b, n = votes.most_common(1)[0]
                    if b != 0 and n >= 0.9 * len(sites):
                        self.hunk_host = base + start
                        self.hunk_amiga = b
                        return True
                i = data.find(b"df0:RoomData", i + 1)
        return False

    def wait_hunk(self, timeout=60):
        t0 = time.time()
        while time.time() - t0 < timeout:
            self.dismiss_dialogs()
            if self.find_hunk():
                return True
            time.sleep(0.5)
        return False

    def read(self, addr, n):
        return self._read_host(self.hunk_host + addr, n)

    def word(self, addr):
        return struct.unpack(">H", self.read(addr, 2))[0]

    def long(self, addr):
        return struct.unpack(">I", self.read(addr, 4))[0]

    def write(self, addr, data):
        self._write_host(self.hunk_host + addr, data)

    def _write_host(self, addr, data):
        n = ctypes.c_size_t()
        kernel32.WriteProcessMemory(self.h, ctypes.c_void_p(addr), data, len(data), ctypes.byref(n))

    # WinUAE maps the Amiga address space linearly, so chip RAM is reachable from the hunk.
    def amiga_read(self, addr, n):
        return self._read_host(self.hunk_host - self.hunk_amiga + addr, n)

    def amiga_write(self, addr, data):
        self._write_host(self.hunk_host - self.hunk_amiga + addr, data)

    # --- input via memory (works without window focus) ----------------------
    # Every joystick/fire read in ns is redirected from the hardware register to a mailbox in
    # unused chip RAM (user vector area). Same opcodes and lengths, only the address changes.
    def _input_sites(self):
        code, _ = _ns_code_and_relocs()
        sites = []
        for i in range(0, len(code) - 8, 2):
            op = struct.unpack(">H", code[i:i + 2])[0]
            if op & 0xF1FF == 0x3039 and code[i + 2:i + 6] == b"\x00\xdf\xf0\x0c":  # move.w $dff00c.l,Dn
                sites.append((i + 2, code[i + 2:i + 6], MAILBOX_JOY))
            if code[i:i + 8] == b"\x08\x39\x00\x07\x00\xbf\xe0\x01":  # btst #7,$bfe001.l
                sites.append((i + 4, code[i + 4:i + 8], MAILBOX_FIRE))
        return sites

    def take_control(self):
        self.set_input()
        for off, orig, mbox in self._input_sites():
            if self.read(off, 4) == orig:
                self.write(off, struct.pack(">I", mbox))
        self.controlled = True

    def release_control(self):
        for off, orig, mbox in self._input_sites():
            if self.read(off, 4) == struct.pack(">I", mbox):
                self.write(off, orig)
        self.controlled = False

    def set_input(self, dirs=(), fire=False):
        """dirs: any of 'up','down','left','right'. Encoded like JOY1DAT / CIA-A PRA bit 7."""
        right, left = "right" in dirs, "left" in dirs
        b1, b9 = int(right), int(left)
        b0 = int("down" in dirs) ^ b1
        b8 = int("up" in dirs) ^ b9
        self.amiga_write(MAILBOX_JOY, struct.pack(">H", b9 << 9 | b8 << 8 | b1 << 1 | b0))
        self.amiga_write(MAILBOX_FIRE, bytes([0x00 if fire else 0x80]))

    def hold(self, dirs=(), fire=False, seconds=0.2):
        self.set_input(dirs, fire)
        time.sleep(seconds)
        self.set_input()

    def quit(self, timeout=15):
        """Close WinUAE gracefully (so statefile_quit is written)."""
        hwnd = self.main_window()
        if hwnd:
            user32.PostMessageW(hwnd, WM_CLOSE, 0, 0)
        t0 = time.time()
        while time.time() - t0 < timeout and _running_pid() == self.pid:
            self.dismiss_dialogs()
            time.sleep(0.3)

    def kill(self):
        if self.proc:
            self.proc.kill()
            self.proc.wait()


def boot(timeout=180):
    """Fresh boot with turbo floppy, press fire through the intro screens, save state in gameplay."""
    if os.path.exists(STATE):
        os.remove(STATE)
    uae = UAE.launch(extra={"floppy_speed": 0, "statefile_quit": STATE})
    t0 = time.time()
    try:
        if not uae.wait_hunk():
            raise RuntimeError("game code not found in memory")
        print(f"{time.time()-t0:5.1f}s game code at ${uae.hunk_amiga:06x}", flush=True)
        uae.take_control()
        while time.time() - t0 < timeout:
            if uae.in_game():
                time.sleep(2)
                uae.release_control()  # the saved state must contain the original code
                uae.screenshot(os.path.join(UAEDIR, "ingame.png"))
                print(f"{time.time()-t0:5.1f}s in gameplay, saving state", flush=True)
                uae.quit()
                print("saved" if os.path.exists(STATE) else "state NOT written", STATE)
                return
            uae.hold(fire=True, seconds=0.2)
            time.sleep(1.3)
        raise RuntimeError("did not reach gameplay")
    finally:
        if _running_pid() == uae.pid:
            uae.kill()


def launch(fresh=False):
    state = None if fresh or not os.path.exists(STATE) else STATE
    uae = UAE.launch(state=state, extra={"floppy_speed": 0})
    uae.wait_hunk(30)
    print(f"WinUAE pid {uae.pid} running" + (" from ingame.uss" if state else " (fresh boot)"))


def watch(addrs):
    uae = UAE.attach()
    if not uae.wait_hunk(30):
        raise RuntimeError("game code not found in memory")
    vars_ = [(int(a, 16), 2, f"${a}") for a in addrs] or DEFAULT_VARS
    last = None
    while _running_pid() == uae.pid:
        vals = tuple(struct.unpack(">H" if s == 2 else ">I", uae.read(a, s))[0] for a, s, _ in vars_)
        if vals != last:
            print(time.strftime("%H:%M:%S"), "  ".join(f"{n}={v}" for (_, _, n), v in zip(vars_, vals)), flush=True)
            last = vals
        time.sleep(0.5)


if __name__ == "__main__":
    cmd = sys.argv[1] if len(sys.argv) > 1 else ""
    if cmd == "boot":
        boot()
    elif cmd == "launch":
        launch("--fresh" in sys.argv)
    elif cmd == "watch":
        watch([a.lstrip("$") for a in sys.argv[2:]])
    elif cmd == "shot":
        u = UAE.attach()
        print(u.screenshot(sys.argv[2]))
    else:
        print(__doc__)
