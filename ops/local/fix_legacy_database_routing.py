import pyodbc

# 1. Connect to master on local SQL Server
conn = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=master;UID=sa;PWD=a8u5qfwr', autocommit=True)
cursor = conn.cursor()

cursor.execute("SELECT name FROM sys.databases WHERE name = 'FazalDinPP19'")
if not cursor.fetchone():
    print("Creating database FazalDinPP19 on SQL Server...")
    cursor.execute("CREATE DATABASE [FazalDinPP19]")

# 2. Connect to FazalDinPP19 and create/copy security tables from FazalDinPP19DataBaseV2
conn2 = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=FazalDinPP19;UID=sa;PWD=a8u5qfwr', autocommit=True)
c2 = conn2.cursor()

tables_to_copy = ['Users', 'UserGroups', 'UserRights', 'Groups', 'UserAuthenticationInfo', 'temp_GroupRights']

for tbl in tables_to_copy:
    c2.execute(f"IF OBJECT_ID('dbo.[{tbl}]', 'U') IS NOT NULL DROP TABLE dbo.[{tbl}]")
    c2.execute(f"SELECT * INTO dbo.[{tbl}] FROM FazalDinPP19DataBaseV2.dbo.[{tbl}]")
    print(f"Copied table {tbl} into FazalDinPP19 successfully.")

print("\n--- FazalDinPP19 Users Count ---")
c2.execute("SELECT COUNT(*) FROM dbo.Users")
print("Users count in FazalDinPP19:", c2.fetchone()[0])
