import pyodbc

conn = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=FazalDinPP19DataBaseV2;UID=sa;PWD=a8u5qfwr')
cursor = conn.cursor()

print("=== 1. ALL TABLES IN FazalDinPP19DataBaseV2 ===")
cursor.execute("SELECT table_name FROM information_schema.tables WHERE table_type = 'BASE TABLE' ORDER BY table_name")
tables = [r[0] for r in cursor.fetchall()]
for t in tables:
    if any(k in t.lower() for k in ['user', 'login', 'sec', 'auth', 'pass', 'admin', 'emp', 'staff']):
        print("MATCHED TABLE:", t)
        cursor.execute(f"SELECT COUNT(*) FROM dbo.[{t}]")
        print(f"  Count: {cursor.fetchone()[0]}")
        cursor.execute(f"SELECT TOP 5 * FROM dbo.[{t}]")
        for r in cursor.fetchall():
            print("  Row:", r)
