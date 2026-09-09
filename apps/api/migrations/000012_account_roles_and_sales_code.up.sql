ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = CASE role
    WHEN 'owner' THEN 'super_admin'
    WHEN 'viewer' THEN 'customer'
    ELSE role
END;
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'customer';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('super_admin', 'admin', 'sales', 'customer'));
ALTER TABLE users ADD COLUMN IF NOT EXISTS sales_code VARCHAR(64);
CREATE UNIQUE INDEX IF NOT EXISTS users_sales_code_unique ON users (sales_code) WHERE sales_code IS NOT NULL;

ALTER TABLE site_access DROP CONSTRAINT IF EXISTS site_access_access_role_check;
UPDATE site_access SET access_role = CASE access_role WHEN 'owner' THEN 'sales' WHEN 'viewer' THEN 'customer' ELSE access_role END;
ALTER TABLE site_access ADD CONSTRAINT site_access_access_role_check CHECK (access_role IN ('sales', 'customer'));
