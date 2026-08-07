import pyodbc

conn = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=FazalDinPP19DataBaseV2;UID=sa;PWD=a8u5qfwr')
cursor = conn.cursor()

cursor.execute("SELECT column_name, collation_name FROM information_schema.columns WHERE table_name = 'Users'")
for r in cursor.fetchall():
    print(r)
