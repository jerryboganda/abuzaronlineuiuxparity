import pyodbc

conn = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=FazalDinPP19DataBaseV2;UID=sa;PWD=a8u5qfwr')
cursor = conn.cursor()

cursor.execute("SELECT UserCode, UserName, Password, Active FROM dbo.Users")
print("--- EXACT USERNAME DETAILS ---")
for r in cursor.fetchall():
    ucode, uname, pwd, act = r
    print(f"UserCode={ucode}: UserName={uname!r} (len={len(uname)}, bytes={uname.encode('utf-8')}), Active={act!r}")
