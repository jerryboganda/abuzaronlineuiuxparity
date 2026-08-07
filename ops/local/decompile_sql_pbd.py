import re

pbd_path = r"d:\ABUZAR\V2_AbuzarSoftware\Application\abuzarapp.pbd"

with open(pbd_path, 'rb') as f:
    data = f.read()

# Search for SELECT statements in utf-16le
text = data.decode('utf-16le', errors='ignore')

matches = re.findall(r'select\s+.*?\s+from\s+.*?(?:where\s+.*?)?(?:;|~|\r|\n|\x00)', text, re.IGNORECASE)
print(f"Total SQL select queries found: {len(matches)}")
for m in matches:
    if any(tbl in m.lower() for tbl in ['users', 'usergroups', 'groups', 'userauthenticationinfo']):
        clean = "".join(c for c in m if 32 <= ord(c) <= 126 or c in '\n\r\t')
        print(f"\n---> {clean.strip()}")
