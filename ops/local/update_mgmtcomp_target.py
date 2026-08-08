config_path = r"d:\ABUZAR\V2_AbuzarSoftware\Application\mgmtcomp.pbd"

with open(config_path, 'rb') as f:
    raw = f.read()

lines = raw.decode('utf-8', errors='ignore').split('\n')

def encode(text):
    return "".join(chr(ord(c) + 128) for c in text)

# Update lines 14 and 15 to 'FazalDinPP19DataBaseV2'
lines[14] = encode('FazalDinPP19DataBaseV2') + '\r'
lines[15] = encode('FazalDinPP19DataBaseV2') + '\r'

new_raw = "\n".join(lines).encode('utf-8')
with open(config_path, 'wb') as f:
    f.write(new_raw)

print("Updated mgmtcomp.pbd database target lines to FazalDinPP19DataBaseV2!")
