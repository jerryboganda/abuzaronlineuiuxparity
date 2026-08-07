import os

app_dir = r"d:\ABUZAR\V2_AbuzarSoftware\Application"

search_str = "Either User name does not exist"

for root, dirs, files in os.walk(app_dir):
    for f in files:
        if f.endswith('.pbd') or f.endswith('.exe') or f.endswith('.dll'):
            path = os.path.join(root, f)
            try:
                with open(path, 'rb') as fp:
                    content = fp.read()
                    if search_str.encode('utf-16le') in content or search_str.encode('utf-8') in content or b'User Validation' in content:
                        print(f"FOUND MATCH IN: {f}")
            except Exception as e:
                pass
