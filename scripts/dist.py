"""Build the files to hand to other people: a zip of the source and a zip of the programs.

    python scripts/dist.py

Run it in the port directory, with the work committed (the source zip is made from HEAD). It
writes to dist/ and checks that nothing of the game's data is in either zip. The programs are
built for the system this runs on: the game needs cgo on Linux and macOS, so build there for
those.
"""
import os
import platform
import subprocess
import sys
import zipfile

HERE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
os.chdir(HERE)
OUT = os.path.join(HERE, "dist")
os.makedirs(OUT, exist_ok=True)

BAD_NAMES = ("assets-local", ".adf", ".uss")


def run(*cmd, **kw):
    r = subprocess.run(cmd, capture_output=True, text=True, **kw)
    if r.returncode:
        sys.exit("failed: %s\n%s%s" % (" ".join(cmd), r.stdout, r.stderr))
    return r.stdout.strip()


def check(path):
    with zipfile.ZipFile(path) as z:
        for n in z.namelist():
            if any(b in n for b in BAD_NAMES):
                sys.exit("%s has %s in it: refusing to hand that out" % (path, n))
    print("ok:", path, "(%d bytes)" % os.path.getsize(path))


# The source: what is committed under port/.
src = os.path.join(OUT, "gnistaengine-src.zip")
top = run("git", "rev-parse", "--show-toplevel")
run("git", "-C", top, "archive", "--format=zip", "--prefix=gnistaengine/", "-o", src, "HEAD" if os.path.samefile(HERE, top) else "HEAD:" + os.path.relpath(HERE, top).replace(os.sep, "/"))
check(src)

# The programs for this system.
goos = run("go", "env", "GOOS")
goarch = run("go", "env", "GOARCH")
ext = ".exe" if goos == "windows" else ""
bin_zip = os.path.join(OUT, "gnistaengine-%s-%s.zip" % (goos, goarch))
with zipfile.ZipFile(bin_zip, "w", zipfile.ZIP_DEFLATED) as z:
    for name in ("game", "extract"):
        exe = os.path.join(OUT, name + ext)
        run("go", "build", "-trimpath", "-o", exe, "./cmd/" + name)
        z.write(exe, "gnistaengine/" + name + ext)
        os.remove(exe)
    z.write("README.md", "gnistaengine/README.md")
    z.write("docs/improvements.md", "gnistaengine/docs/improvements.md")
check(bin_zip)
print("Give people these, not assets-local: they need their own copy of the disk (see README.md).")
