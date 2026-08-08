import pyodbc

conn = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=master;UID=sa;PWD=a8u5qfwr', autocommit=True)
cursor = conn.cursor()

cursor.execute("SELECT name FROM sys.databases WHERE name = 'FazalDinPP19'")
if not cursor.fetchone():
    print("Database FazalDinPP19 does not exist! Creating database FazalDinPP19...")
    # We can create DB or check mgmtcomp.pbd lines
