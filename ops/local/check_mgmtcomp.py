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

print(f"Total lines: {len(lines)}")
for idx, l in enumerate(lines):
    dec = decode(l)
    clean_dec = dec.encode('ascii', errors='ignore').decode('ascii')
    print(f"Line {idx:2d}: {clean_dec!r}")
