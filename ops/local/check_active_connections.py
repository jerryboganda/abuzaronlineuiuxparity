import pyodbc

conn = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=master;UID=sa;PWD=a8u5qfwr')
cursor = conn.cursor()

cursor.execute("""
    SELECT s.session_id, s.login_name, s.host_name, s.program_name, db_name(s.database_id) as dbname
    FROM sys.dm_exec_sessions s
    WHERE s.is_user_process = 1
""")
rows = cursor.fetchall()
print("=== ACTIVE SQL SERVER USER SESSIONS ===")
for r in rows:
    print(r)
