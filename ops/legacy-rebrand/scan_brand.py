"""Scan legacy PowerBuilder binaries for branding strings (read-only)."""
import os
import re
import sys
import json

ROOT = sys.argv[1] if len(sys.argv) > 1 else r"D:\ABUZAR\V2_AbuzarSoftware\Application"
TERMS = ["abuzar", "waseela"]

SKIP_EXT = {".cat", ".wav", ".bmp", ".png", ".ico", ".chm", ".hlp"}


def find_all(hay: bytes, needle: bytes):
    out = []
    start = 0
    while True:
        i = hay.find(needle, start)
        if i < 0:
            return out
        out.append(i)
        start = i + 1


def context(data: bytes, off: int, enc: str, back=60, fwd=90):
    lo = max(0, off - back)
    chunk = data[lo:off + fwd]
    if enc == "utf16":
        try:
            txt = chunk.decode("utf-16le", errors="replace")
        except Exception:
            txt = ""
    else:
        txt = chunk.decode("latin-1", errors="replace")
    return re.sub(r"[^\x20-\x7e]+", "\u00b7", txt)


results = {}
total = 0
for dirpath, dirnames, filenames in os.walk(ROOT):
    for fn in filenames:
        ext = os.path.splitext(fn)[1].lower()
        if ext in SKIP_EXT:
            continue
        path = os.path.join(dirpath, fn)
        try:
            if os.path.getsize(path) > 300 * 1024 * 1024:
                continue
            with open(path, "rb") as fh:
                data = fh.read()
        except Exception:
            continue

        hits = []
        low = data.lower()
        for term in TERMS:
            # ASCII / ANSI
            for off in find_all(low, term.encode("ascii")):
                hits.append({
                    "term": term, "enc": "ascii", "off": off,
                    "raw": data[off:off + len(term)].decode("latin-1"),
                    "ctx": context(data, off, "ascii"),
                })
            # UTF-16LE
            u16 = term.encode("utf-16le")
            for off in find_all(low, u16):
                hits.append({
                    "term": term, "enc": "utf16", "off": off,
                    "raw": data[off:off + len(u16)].decode("utf-16le", errors="replace"),
                    "ctx": context(data, off, "utf16"),
                })
        if hits:
            rel = os.path.relpath(path, ROOT)
            results[rel] = hits
            total += len(hits)

summary = sorted(((len(v), k) for k, v in results.items()), reverse=True)
print("SCAN ROOT:", ROOT)
print("FILES WITH HITS:", len(results), " TOTAL HITS:", total)
print("-" * 70)
for n, name in summary:
    encs = {}
    for h in results[name]:
        encs[(h["term"], h["enc"])] = encs.get((h["term"], h["enc"]), 0) + 1
    detail = ", ".join(f"{t}/{e}={c}" for (t, e), c in sorted(encs.items()))
    print(f"{n:6d}  {name}   [{detail}]")

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "scan_brand.json")
with open(out, "w", encoding="utf-8") as fh:
    json.dump(results, fh, indent=1)
print("-" * 70)
print("JSON written:", out)
