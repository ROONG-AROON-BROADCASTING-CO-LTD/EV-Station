CREATE TABLE line_customer_profiles (
    line_user_id TEXT PRIMARY KEY,
    contact_name VARCHAR(160) NOT NULL,
    contact_phone VARCHAR(40) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Existing LINE submissions already hold a contact snapshot on their site.
-- Seed one latest profile per LINE user so existing customers do not need to
-- enter the same details again on their first visit after this release.
INSERT INTO line_customer_profiles (line_user_id, contact_name, contact_phone, created_at, updated_at)
SELECT DISTINCT ON (ls.line_user_id)
    ls.line_user_id,
    s.contact_name,
    s.contact_phone,
    s.created_at,
    s.updated_at
FROM line_submissions ls
JOIN sites s ON s.id = ls.site_id
WHERE btrim(s.contact_name) <> ''
  AND btrim(s.contact_phone) <> ''
ORDER BY ls.line_user_id, s.updated_at DESC;
