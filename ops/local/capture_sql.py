import pyodbc
import time

conn = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=FazalDinPP19DataBaseV2;UID=sa;PWD=a8u5qfwr')
cursor = conn.cursor()

print("Monitoring recent SQL queries executed on SQL Server for 15 seconds...")
seen = set()

for i in range(30):
    cursor.execute("""
        SELECT dest.text, stats.last_execution_time 
        FROM sys.dm_exec_query_stats AS stats 
        CROSS APPLY sys.dm_exec_sql_text(stats.sql_handle) AS dest 
        ORDER BY stats.last_execution_time DESC
    """)
    rows = cursor.fetchmany(10)
    for txt, tstamp in rows:
        key = (txt, str(tstamp))
        if key not in seen and 'sys.' not in txt.lower():
            seen.add(key)
            print(f"\n--- [{tstamp}] ---")
            print(txt.strip())
    time.sleep(0.5)
