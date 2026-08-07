import os

config_path = r"d:\ABUZAR\V2_AbuzarSoftware\Application\mgmtcomp.pbd"
with open(config_path, 'rb') as f:
    lines = f.read().decode('utf-8', errors='ignore').split('\n')

def decode(line):
    chars = []
    for c in line.rstrip('\r'):
        val = ord(c)
        if val >= 128:
            chars.append(chr(val - 128))
        else:
            chars.append(c)
    return "".join(chars)

print(f"Total lines in mgmtcomp.pbd: {len(lines)}")
for idx, line in enumerate(lines):
    if idx in [13, 14, 15, 16, 17, 18, 19]:
        print(f"Line {idx}: raw={line.strip()!r} | decoded={decode(line)!r}")
