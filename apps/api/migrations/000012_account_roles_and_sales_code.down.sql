ALTER TABLE site_access DROP CONSTRAINT IF EXISTS site_access_access_role_check;
ALTER TABLE site_access ADD CONSTRAINT site_access_access_role_check CHECK (access_role IN ('owner', 'sales', 'viewer'));
DROP INDEX IF EXISTS users_sales_code_unique;
ALTER TABLE users DROP COLUMN IF EXISTS sales_code;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = CASE role WHEN 'super_admin' THEN 'owner' WHEN 'customer' THEN 'viewer' ELSE role END;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('owner', 'sales', 'viewer'));
