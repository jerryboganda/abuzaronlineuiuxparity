import pyodbc

conn1 = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=FazalDinPP19DataBaseV2;UID=sa;PWD=a8u5qfwr')
conn2 = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=AbuzarLegacyReference;UID=sa;PWD=a8u5qfwr')

print("--- FazalDinPP19DataBaseV2: UserGroups ---")
c1 = conn1.cursor()
c1.execute("SELECT * FROM dbo.UserGroups")
for r in c1.fetchall():
    print(r)

print("\n--- AbuzarLegacyReference: UserGroups ---")
c2 = conn2.cursor()
c2.execute("SELECT * FROM dbo.UserGroups")
for r in c2.fetchall():
    print(r)

print("\n--- AbuzarLegacyReference: UserAuthenticationInfo ---")
c2.execute("SELECT * FROM dbo.UserAuthenticationInfo")
for r in c2.fetchall():
    print(r)
