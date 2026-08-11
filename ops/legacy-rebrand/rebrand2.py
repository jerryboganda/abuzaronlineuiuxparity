"""Rebrand the legacy PowerBuilder app: Abuzar/Waseela -> Bismillah Pharmacy Software.

Binary files (.pbd/.exe/.ext/.dll/.mdb/.doc): strict SAME-LENGTH in-place patching.
PowerBuilder PBD string pools are length-prefixed (4-byte LE = chars*2+2), so file
size must never change. Shorter replacements are space-padded to the exact length.

Text files: free-form replacement (no length constraint).
"""
import os
import re
import sys

BIN_EXT = {".pbd", ".exe", ".ext", ".dll", ".mdb", ".doc", ".apl", ".ocx",
           ".pbx", ".flt"}
TEXT_EXT = {".json", ".ps1", ".md", ".sql", ".txt", ".ini", ".bat", ".cmd",
            ".xml", ".csv", ".log", ".py", ".yml", ".yaml", ".vbs", ".reg"}
# Database/archive payloads: never byte-patch these (corruption risk, and they
# are regenerated artefacts rather than program code).
DANGEROUS_EXT = {".mdf", ".ldf", ".ndf", ".trn", ".rat", ".ims", ".zip", ".7z",
                 ".cab", ".msi", ".iso", ".gz", ".rar", ".vhd", ".dmp"}

# Case-SENSITIVE phrase pairs, longest first. Must be equal length.
PHRASES = [
    ("mailto:abuzarconsultancy@yahoo.com", "mailto:bismillahpharmacy@yahoo.com"),
    ("abuzarconsultancy@yahoo.com",        "bismillahpharmacy@yahoo.com"),
    ("mailto:info@abuzar.com.pk",          "mailto:info@bismillah.com"),
    ("http://www.abuzar.com.pk",           "http://www.bismillah.com"),
    ("Waseela Application",                "Bismillah Pharmacy "),
    ("WASEELA APPLICATION",                "BISMILLAH PHARMACY "),
    ("waseela application",                "bismillah pharmacy "),
    ("ABUZAR CONSULTANCY",                 "BISMILLAH PHARMACY"),
    ("Abuzar Consultancy",                 "Bismillah Pharmacy"),
    ("abuzar consultancy",                 "bismillah pharmacy"),
    ("ABUZAR Consultancy",                 "BISMILLAH Pharmacy"),
    ("info@abuzar.com.pk",                 "info@bismillah.com"),
    ("WASEELA   ",                         "BISMILLAH "),
    ("Waseela - ",                         "Bismillah "),
]
# Case-preserving token fallback (pure alphabetic words only).
TOKENS = [("abuzar", "bismil"), ("waseela", "bismila")]

for a, b in PHRASES + TOKENS:
    assert len(a) == len(b), (a, b, len(a), len(b))
for a, b in TOKENS:
    assert a.isalpha() and b.isalpha(), (a, b)


def match_case(src: str, dst: str) -> str:
    return "".join(d.upper() if s.isupper() else d.lower()
                   for s, d in zip(src, dst))


def token_variants(a: str, b: str):
    """All case variants of an alphabetic token, with matching replacement."""
    out = []
    for va in {a.lower(), a.upper(), a.capitalize()}:
        out.append((va, match_case(va, b)))
    return out


def all_pairs():
    pairs = list(PHRASES)
    for a, b in TOKENS:
        pairs.extend(token_variants(a, b))
    return pairs


def patch_binary(path, dry):
    with open(path, "rb") as fh:
        data = bytearray(fh.read())
    orig_len = len(data)
    total = 0
    for va, vb in all_pairs():
        assert len(va) == len(vb)
        for enc in ("utf-16le", "latin-1"):
            try:
                ba, bb = va.encode(enc), vb.encode(enc)
            except Exception:
                continue
            if len(ba) != len(bb):
                continue
            start = 0
            while True:
                i = data.find(ba, start)
                if i < 0:
                    break
                data[i:i + len(ba)] = bb
                start = i + len(bb)
                total += 1
    assert len(data) == orig_len, "SIZE CHANGED"
    if total and not dry:
        with open(path, "wb") as fh:
            fh.write(bytes(data))
    return total


def rebrand_text(text):
    n = 0
    for a, b in PHRASES:
        text, k = text.replace(a, b), text.count(a)
        n += k
    for a, b in TOKENS:
        pat = re.compile(re.escape(a), re.IGNORECASE)
        text, k = pat.subn(lambda m, b=b: match_case(m.group(0), b), text)
        n += k
    return text, n


def patch_text(path, dry):
    raw = open(path, "rb").read()
    # Preserve the file's original BOM state exactly.
    if raw.startswith(b"\xef\xbb\xbf"):
        enc = "utf-8-sig"
    elif raw.startswith(b"\xff\xfe") or raw.startswith(b"\xfe\xff"):
        enc = "utf-16"
    else:
        enc = None
        for cand in ("utf-8", "latin-1"):
            try:
                raw.decode(cand)
                enc = cand
                break
            except Exception:
                continue
        if enc is None:
            return 0
    try:
        text = raw.decode(enc)
    except Exception:
        return 0
    new, n = rebrand_text(text)
    if n and not dry:
        out = new.encode(enc)
        assert not (enc == "utf-8" and out.startswith(b"\xef\xbb\xbf"))
        with open(path, "wb") as fh:
            fh.write(out)
    return n


def main():
    root = sys.argv[1]
    dry = "--apply" not in sys.argv
    grand, touched = 0, []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if "_RENAME_BACKUP" not in d]
        for fn in filenames:
            path = os.path.join(dirpath, fn)
            ext = os.path.splitext(fn)[1].lower()
            try:
                size = os.path.getsize(path)
            except OSError:
                continue
            if size > 300 * 1024 * 1024:
                continue
            if ext in DANGEROUS_EXT:
                continue
            # SQL Server backups share the .bak suffix with PowerBuilder backups.
            if ext == ".bak" and not fn.lower().endswith(".pbd.bak"):
                continue
            try:
                if ext in TEXT_EXT:
                    n = patch_text(path, dry)
                elif ext in BIN_EXT or size < 80 * 1024 * 1024:
                    n = patch_binary(path, dry)
                else:
                    continue
            except AssertionError as e:
                print("!! ABORT(size)", path, e)
                continue
            except Exception as e:
                print("!! ERROR", path, type(e).__name__, e)
                continue
            if n:
                grand += n
                touched.append((n, os.path.relpath(path, root)))
    touched.sort(reverse=True)
    for n, rel in touched:
        print(f"{n:6d}  {rel}")
    print("-" * 60)
    print(("DRY-RUN " if dry else "APPLIED ")
          + f"files={len(touched)} replacements={grand}")


if __name__ == "__main__":
    main()
