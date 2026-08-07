SET app.authenticating = 'true';

-- Set bcrypt password hash for ADMIN and legacy users in abuzarnext
UPDATE users 
SET password_hash = crypt('pakistan9080', gen_salt('bf'))
WHERE username = 'ADMIN';

UPDATE users 
SET password_hash = crypt('1', gen_salt('bf'))
WHERE username = 'RAEES KHAN';

UPDATE users 
SET password_hash = crypt('55', gen_salt('bf'))
WHERE username = 'DR SAIRA';

UPDATE users 
SET password_hash = crypt('z0', gen_salt('bf'))
WHERE username = 'ZUBAIR ARIF';

UPDATE users 
SET password_hash = crypt('25', gen_salt('bf'))
WHERE username = 'SHAZIB';

UPDATE users 
SET password_hash = crypt('3', gen_salt('bf'))
WHERE username = 'HAMMAD';

UPDATE users 
SET password_hash = crypt('60', gen_salt('bf'))
WHERE username IN ('HAMID ALI', 'FARYAD');

UPDATE users 
SET password_hash = crypt('0', gen_salt('bf'))
WHERE username = 'ALI';
