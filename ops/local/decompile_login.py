pbd_path = r"d:\ABUZAR\V2_AbuzarSoftware\Application\abuzarapp.pbd"

with open(pbd_path, 'rb') as f:
    data = f.read()

target = "Either User name does not exist"
idx = data.find(target.encode('utf-16le'))
print(f"Found target at byte offset: {idx}")

start = max(0, idx - 4000)
end = min(len(data), idx + 4000)
snippet = data[start:end]

# Try decoding utf-16le
text = snippet.decode('utf-16le', errors='ignore')

lines = text.split('\x00')
for line in text.splitlines():
    clean = "".join(c for c in line if 32 <= ord(c) <= 126 or c in '\n\r\t')
    if len(clean.strip()) > 3:
        if any(kw in clean.lower() for kw in ['select', 'from', 'where', 'user', 'active', 'password', 'login', 'valid', 'update', 'insert', 'group', 'dbo', 'either']):
            print(clean.strip())
