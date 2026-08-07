import pyodbc

conn = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=FazalDinPP19DataBaseV2;UID=sa;PWD=a8u5qfwr')
cursor = conn.cursor()

print("--- GROUPS TABLE COLUMNS ---")
cursor.execute("SELECT column_name FROM information_schema.columns WHERE table_name = 'Groups'")
print([r[0] for r in cursor.fetchall()])

print("\n--- GROUPS TABLE ROWS ---")
cursor.execute("SELECT * FROM dbo.Groups")
for r in cursor.fetchall():
    print(r)
