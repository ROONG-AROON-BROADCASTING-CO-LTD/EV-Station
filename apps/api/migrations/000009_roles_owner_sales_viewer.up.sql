ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = CASE role WHEN 'admin' THEN 'owner' WHEN 'customer' THEN 'viewer' ELSE role END;
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'viewer';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('owner', 'sales', 'viewer'));
