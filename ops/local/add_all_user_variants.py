import pyodbc

conn = pyodbc.connect('DRIVER={ODBC Driver 17 for SQL Server};SERVER=127.0.0.1;DATABASE=FazalDinPP19DataBaseV2;UID=sa;PWD=a8u5qfwr')
cursor = conn.cursor()

variants = [
    ('admin', 'pakistan9080', 2),
    ('Admin', 'pakistan9080', 2),
    ('administrator', 'pakistan9080', 2),
    ('Administrator', 'pakistan9080', 2),
    ('ADMINISTRATOR', 'pakistan9080', 2),
    ('sa', 'pakistan9080', 2),
    ('SA', 'pakistan9080', 2),
    ('raees khan', '1', 12),
    ('Raees Khan', '1', 12),
    ('dr saira', '55', 11),
    ('Dr Saira', '55', 11),
    ('zubair arif', 'z0', 12),
    ('Zubair Arif', 'z0', 12),
    ('shazib', '25', 11),
    ('Shazib', '25', 11),
    ('hammad', '3', 12),
    ('Hammad', '3', 12),
    ('hamid ali', '60', 12),
    ('Hamid Ali', '60', 12),
    ('ali', '0', 12),
    ('Ali', '0', 12),
    ('faryad', '60', 12),
    ('Faryad', '60', 12),
]

cursor.execute("SELECT MAX(UserCode) FROM dbo.Users")
max_code = cursor.fetchone()[0] or 10

for uname, pwd, gcode in variants:
    cursor.execute("SELECT UserCode FROM dbo.Users WHERE UserName = ?", (uname,))
    row = cursor.fetchone()
    if not row:
        max_code += 1
        print(f"Adding user variant: UserCode={max_code}, UserName={uname!r}, Password={pwd!r}")
        cursor.execute("INSERT INTO dbo.Users (UserCode, UserName, Password, Active) VALUES (?, ?, ?, 'Y')", (max_code, uname, pwd))
        cursor.execute("INSERT INTO dbo.UserGroups (UserCode, GroupCode) VALUES (?, ?)", (max_code, gcode))

conn.commit()
print("All user variants synchronized successfully!")
