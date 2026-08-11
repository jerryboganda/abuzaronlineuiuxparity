"""Scan FazalDinPP19DataBaseV2 for every trace of Abuzar/Waseela.

Covers: object names, module (proc/view/fn/trigger) bodies, column names,
and the data in every character column of every user table.
"""
import os
import re
import subprocess
import sys

SQLCMD = r"C:\Program Files\Microsoft SQL Server\Client SDK\ODBC\170\Tools\Binn\SQLCMD.EXE"
DB = "FazalDinPP19DataBaseV2"


def password():
    ini = open(r"D:\ABUZAR\Preferences.ini", "r", errors="ignore").read()
    return re.search(r"(?im)^\s*LogPassword\s*=\s*(.+?)\s*$", ini).group(1)


def q(sql, db=DB, sep="|"):
    p = subprocess.run(
        [SQLCMD, "-S", "127.0.0.1", "-U", "sa", "-P", password(), "-d", db,
         "-h", "-1", "-W", "-s", sep, "-t", "0", "-Q",
         "SET NOCOUNT ON;\n" + sql],
        capture_output=True, text=True, errors="replace")
    if p.returncode != 0:
        return [("ERROR: " + (p.stdout + p.stderr).strip(),)]
    rows = []
    for line in p.stdout.splitlines():
        line = line.rstrip()
        if not line or set(line) <= set("- "):
            continue
        rows.append(tuple(c.strip() for c in line.split(sep)))
    return rows


def section(title, rows):
    print("\n" + "=" * 70)
    print(title, f"({len(rows)})")
    print("=" * 70)
    for r in rows:
        print("  " + " | ".join(r))


LIKE = "(o.name LIKE '%abuzar%' OR o.name LIKE '%waseela%')"

section("OBJECT NAMES", q(f"""
SELECT o.type_desc, s.name+'.'+o.name FROM sys.objects o
JOIN sys.schemas s ON s.schema_id=o.schema_id WHERE {LIKE} ORDER BY 1,2;"""))

section("COLUMN NAMES", q("""
SELECT t.name, c.name FROM sys.columns c JOIN sys.tables t ON t.object_id=c.object_id
WHERE c.name LIKE '%abuzar%' OR c.name LIKE '%waseela%' ORDER BY 1,2;"""))

section("MODULES CONTAINING BRAND", q("""
SELECT o.type_desc, OBJECT_NAME(m.object_id),
  (LEN(m.definition)-LEN(REPLACE(LOWER(m.definition),'abuzar','')))/6
 +(LEN(m.definition)-LEN(REPLACE(LOWER(m.definition),'waseela','')))/7
FROM sys.sql_modules m JOIN sys.objects o ON o.object_id=m.object_id
WHERE m.definition LIKE '%abuzar%' OR m.definition LIKE '%waseela%' ORDER BY 2;"""))

section("OTHER: indexes/keys/triggers/defaults", q("""
SELECT 'INDEX', i.name FROM sys.indexes i WHERE i.name LIKE '%abuzar%' OR i.name LIKE '%waseela%'
UNION ALL SELECT 'SYNONYM', name FROM sys.synonyms WHERE name LIKE '%abuzar%' OR name LIKE '%waseela%'
UNION ALL SELECT 'USER', name FROM sys.database_principals WHERE name LIKE '%abuzar%' OR name LIKE '%waseela%';"""))

# ---- data scan ----
print("\n" + "=" * 70)
print("DATA SCAN (every char column of every user table)")
print("=" * 70)
gen = q("""
SELECT 'SELECT '''+t.name+''','''+c.name+''',COUNT(*) FROM ['+t.name+'] WITH (NOLOCK) WHERE ['
 +c.name+'] LIKE ''%abuzar%'' OR ['+c.name+'] LIKE ''%waseela%'' HAVING COUNT(*)>0;'
FROM sys.columns c JOIN sys.tables t ON t.object_id=c.object_id
JOIN sys.types ty ON ty.user_type_id=c.user_type_id
WHERE ty.name IN ('char','varchar','nchar','nvarchar','text','ntext')
  AND t.is_ms_shipped=0 ORDER BY t.name,c.name;""")
stmts = [r[0] for r in gen if r and r[0].startswith("SELECT")]
print(f"probing {len(stmts)} character columns ...")
batch, hits = [], []
for i in range(0, len(stmts), 120):
    out = q("\n".join(stmts[i:i + 120]))
    for r in out:
        if len(r) == 3 and r[2].isdigit():
            hits.append(r)
        elif r and r[0].startswith("ERROR"):
            print("   ", r[0][:200])
hits.sort(key=lambda x: -int(x[2]))
print(f"\nCOLUMNS WITH BRAND DATA: {len(hits)}")
for t, c, n in hits:
    print(f"  {int(n):8d}  {t}.{c}")
