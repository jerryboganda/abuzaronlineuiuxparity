"""Classify branding hits: extract the full UTF-16LE string around each offset."""
import json
import os
import re
import sys
from collections import Counter, defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = r"D:\ABUZAR\V2_AbuzarSoftware\Application"

with open(os.path.join(HERE, "scan_brand.json"), encoding="utf-8") as fh:
    results = json.load(fh)

PRINTABLE = set(range(0x20, 0x7F)) | {0x09}


def extract_u16_string(data: bytes, off: int, maxlen=400):
    """Walk outward from off while bytes look like printable UTF-16LE."""
    start = off
    while start - 2 >= 0:
        lo, hi = data[start - 2], data[start - 1]
        if hi != 0 or lo not in PRINTABLE:
            break
        start -= 2
        if off - start > maxlen:
            break
    end = off
    n = len(data)
    while end + 1 < n:
        lo, hi = data[end], data[end + 1]
        if hi != 0 or lo not in PRINTABLE:
            break
        end += 2
        if end - off > maxlen:
            break
    return data[start:end].decode("utf-16le", errors="replace"), start, end


strings_by_file = defaultdict(list)
global_counter = Counter()
occurrences = defaultdict(list)

cache = {}
for rel, hits in results.items():
    path = os.path.join(ROOT, rel)
    if path not in cache:
        with open(path, "rb") as fh:
            cache[path] = fh.read()
    data = cache[path]
    seen_spans = set()
    for h in hits:
        if h["enc"] != "utf16":
            continue
        s, a, b = extract_u16_string(data, h["off"])
        if (a, b) in seen_spans:
            continue
        seen_spans.add((a, b))
        s = s.strip()
        strings_by_file[rel].append((s, a, b))
        global_counter[s] += 1
        occurrences[s].append((rel, a, b))
    cache.pop(path, None)

print("DISTINCT BRAND-BEARING STRINGS:", len(global_counter))
print("=" * 100)
for s, n in global_counter.most_common():
    files = sorted({r for r, _, _ in occurrences[s]})
    shown = ", ".join(files[:4]) + (f" +{len(files)-4} more" if len(files) > 4 else "")
    print(f"[{n:4d}x len={len(s):3d}] {s!r}")
    print(f"          files: {shown}")

with open(os.path.join(HERE, "brand_strings.json"), "w", encoding="utf-8") as fh:
    json.dump({s: occurrences[s] for s in global_counter}, fh, indent=1)
print("=" * 100)
print("written brand_strings.json")
