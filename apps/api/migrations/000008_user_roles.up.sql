ALTER TABLE users
    ADD COLUMN IF NOT EXISTS display_name VARCHAR(160) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS role VARCHAR(16) NOT NULL DEFAULT 'viewer',
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('owner', 'sales', 'viewer'));

CREATE TABLE site_access (
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_role VARCHAR(16) NOT NULL CHECK (access_role IN ('owner', 'sales', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (site_id, user_id)
);
CREATE INDEX site_access_user_idx ON site_access (user_id);
