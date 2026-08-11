"""Rebrand the FazalDinPP19DataBaseV2 schema + branding data.

Renames every Waseela/Abuzar SQL object using the SAME token map used for the
PBD binaries so the compiled app keeps resolving names:
    abuzar -> bismil      waseela -> bismila
Strategy: create patched copies under the new names first, verify, then drop
the originals. Same-named modules that merely mention the brand are ALTERed.

Deliberately NOT touched: SaleLedger.Remarks (person name in financial records).
"""
import os
import re
import subprocess
import sys

SQLCMD = r"C:\Program Files\Microsoft SQL Server\Client SDK\ODBC\170\Tools\Binn\SQLCMD.EXE"
DB = "FazalDinPP19DataBaseV2"
OUT = r"D:\ABUZAR\_RENAME_BACKUP_20260812\db\modules"
TOKENS = [("abuzar", "bismil"), ("waseela", "bismila")]
APPLY = "--apply" in sys.argv


def password():
    ini = open(r"D:\ABUZAR\Preferences.ini", "r", errors="ignore").read()
    return re.search(r"(?im)^\s*LogPassword\s*=\s*(.+?)\s*$", ini).group(1)


PW = password()


def run(sql, tsv=False, db=DB):
    args = [SQLCMD, "-S", "127.0.0.1", "-U", "sa", "-P", PW, "-d", db,
            "-b", "-t", "0", "-h", "-1", "-W", "-s", "|", "-Q",
            "SET NOCOUNT ON;\n" + sql]
    return subprocess.run(args, capture_output=True, text=True, errors="replace")


def get_definition(name):
    """sqlcmd -W truncates; -y 0 returns the full module text."""
    tmp = os.path.join(OUT, "_def.txt")
    args = [SQLCMD, "-S", "127.0.0.1", "-U", "sa", "-P", PW, "-d", DB,
            "-b", "-t", "0", "-y", "0", "-o", tmp, "-Q",
            "SET NOCOUNT ON; SELECT definition FROM sys.sql_modules "
            f"WHERE object_id=OBJECT_ID('dbo.[{name}]');"]
    p = subprocess.run(args, capture_output=True, text=True, errors="replace")
    if p.returncode != 0:
        return None
    txt = open(tmp, "r", encoding="utf-8", errors="replace").read()
    return txt.replace("\x00", "").rstrip()


def run_file(path, db=DB):
    args = [SQLCMD, "-S", "127.0.0.1", "-U", "sa", "-P", PW, "-d", db,
            "-b", "-t", "0", "-i", path]
    return subprocess.run(args, capture_output=True, text=True, errors="replace")


def match_case(src, dst):
    return "".join(d.upper() if s.isupper() else d.lower()
                   for s, d in zip(src, dst))


def rebrand(text):
    # Folder-name phrase first so paths stay consistent with the renamed
    # V2_BismillahSoftware directory instead of the shorter token form.
    for a, b in (("AbuzarSoftware", "BismillahSoftware"),):
        text = re.compile(re.escape(a), re.I).sub(b, text)
    for a, b in TOKENS:
        text = re.compile(re.escape(a), re.I).sub(
            lambda m, b=b: match_case(m.group(0), b), text)
    return text


os.makedirs(OUT, exist_ok=True)

# ---------------------------------------------------------------- fetch list
r = run("""
SELECT o.type_desc, o.name
FROM sys.sql_modules m JOIN sys.objects o ON o.object_id=m.object_id
WHERE m.definition LIKE '%abuzar%' OR m.definition LIKE '%waseela%'
   OR o.name LIKE '%abuzar%' OR o.name LIKE '%waseela%'
ORDER BY CASE o.type_desc WHEN 'VIEW' THEN 0 ELSE 1 END, o.name;""")
mods = []
for line in r.stdout.splitlines():
    parts = line.rstrip().split("|")
    if len(parts) == 2 and parts[0].strip() in ("VIEW", "SQL_STORED_PROCEDURE",
                                                "SQL_SCALAR_FUNCTION",
                                                "SQL_TABLE_VALUED_FUNCTION",
                                                "SQL_TRIGGER"):
        mods.append((parts[0].strip(), parts[1].strip()))
print(f"modules to process: {len(mods)}")
if not mods:
    print(r.stdout[-1500:], r.stderr[-500:])

# ------------------------------------------------------- export + patch each
created, altered, dropped = [], [], []
for kind, name in mods:
    body = get_definition(name)
    if not body or "CREATE" not in body.upper():
        print("  !! could not read definition:", name)
        continue
    open(os.path.join(OUT, f"{name}.ORIGINAL.sql"), "w",
         encoding="utf-8", errors="replace").write(body)
    newbody = rebrand(body)
    newname = rebrand(name)
    open(os.path.join(OUT, f"{newname}.PATCHED.sql"), "w",
         encoding="utf-8", errors="replace").write(newbody)
    if newname != name:
        created.append((kind, name, newname, newbody))
    elif newbody != body:
        altered.append((kind, name, newbody))

print(f"  rename+recreate: {len(created)}   alter-in-place: {len(altered)}")
for k, o, n, _ in created:
    print(f"    {k:22} {o}  ->  {n}")
for k, n, _ in altered:
    print(f"    {k:22} {n}  (body only)")

if not APPLY:
    print("\nDRY-RUN - nothing executed. Re-run with --apply")
    sys.exit(0)

# --------------------------------------------------------------- 1. create new
print("\n[1] creating renamed objects ...")
for kind, old, new, body in created:
    p = os.path.join(OUT, f"_exec_{new}.sql")
    open(p, "w", encoding="utf-8", errors="replace").write(body + "\nGO\n")
    res = run_file(p)
    print(("   OK  " if res.returncode == 0 else "   FAIL") + f" {new}")
    if res.returncode != 0:
        print("      ", (res.stdout + res.stderr).strip()[:400])

# --------------------------------------------------------------- 2. alter same
print("\n[2] altering same-named modules ...")
for kind, name, body in altered:
    alt = re.sub(r"(?is)^\s*CREATE\s+(PROC(EDURE)?|VIEW|FUNCTION|TRIGGER)",
                 lambda m: "ALTER " + m.group(1), body, count=1)
    p = os.path.join(OUT, f"_exec_alter_{name}.sql")
    open(p, "w", encoding="utf-8", errors="replace").write(alt + "\nGO\n")
    res = run_file(p)
    print(("   OK  " if res.returncode == 0 else "   FAIL") + f" {name}")
    if res.returncode != 0:
        print("      ", (res.stdout + res.stderr).strip()[:400])

# --------------------------------------------------------------- 3. drop old
print("\n[3] dropping original objects ...")
for kind, old, new, _ in reversed(created):
    verb = "VIEW" if kind == "VIEW" else (
        "PROCEDURE" if "PROCEDURE" in kind else "FUNCTION")
    res = run(f"DROP {verb} dbo.[{old}];")
    print(("   OK  " if res.returncode == 0 else "   FAIL") + f" {old}")
    if res.returncode != 0:
        print("      ", (res.stdout + res.stderr).strip()[:300])

# ------------------------------------------------- 4. column + constraint + data
print("\n[4] renaming column / constraint and updating branding rows ...")
tail = r"""
IF COL_LENGTH('dbo.CustomerDeploymentDetail','Waseela') IS NOT NULL
    EXEC sp_rename 'dbo.CustomerDeploymentDetail.Waseela', 'Bismila', 'COLUMN';
IF OBJECT_ID('dbo.DF_CustomerDeploymentDetail_Waseela') IS NOT NULL
    EXEC sp_rename 'dbo.DF_CustomerDeploymentDetail_Waseela',
                   'DF_CustomerDeploymentDetail_Bismila', 'OBJECT';

UPDATE Preferences SET AppName = 'Bismillah Pharmacy Software'
 WHERE AppName LIKE '%abuzar%' OR AppName LIKE '%waseela%';

UPDATE PreferencesCategory SET Name = REPLACE(REPLACE(Name,'Waseela','Bismillah'),'Abuzar','Bismillah')
 WHERE Name LIKE '%abuzar%' OR Name LIKE '%waseela%';

UPDATE ConfigSetting SET Name = REPLACE(REPLACE(Name,'Waseela','Bismila'),'Abuzar','Bismil')
 WHERE Name LIKE '%abuzar%' OR Name LIKE '%waseela%';

UPDATE SoftwarePreferences SET Name = REPLACE(REPLACE(Name,'Waseela','Bismila'),'Abuzar','Bismil')
 WHERE Name LIKE '%abuzar%' OR Name LIKE '%waseela%';

UPDATE SoftwarePreferences
   SET PrefValue  = 'PHARMACY SOFTWARE V3 01.01.2025',
       PrefValue2 = 'PHARMACY SOFTWARE V3 01.01.2025'
 WHERE Name = 'appname';

UPDATE SoftwarePreferences
   SET PrefValue  = REPLACE(REPLACE(PrefValue ,'V3_AbuzarSoftware','V2_BismillahSoftware'),'V2_AbuzarSoftware','V2_BismillahSoftware'),
       PrefValue2 = REPLACE(REPLACE(PrefValue2,'V3_AbuzarSoftware','V2_BismillahSoftware'),'V2_AbuzarSoftware','V2_BismillahSoftware')
 WHERE PrefValue LIKE '%AbuzarSoftware%' OR PrefValue2 LIKE '%AbuzarSoftware%';
"""
p = os.path.join(OUT, "_exec_tail.sql")
open(p, "w", encoding="utf-8").write(tail + "\nGO\n")
res = run_file(p)
print(res.stdout.strip()[-2000:])
if res.returncode != 0:
    print("FAIL:", (res.stdout + res.stderr).strip()[:800])
print("\nDONE")
