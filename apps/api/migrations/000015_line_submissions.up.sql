CREATE TABLE line_submissions (
 site_id uuid PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
 line_user_id text NOT NULL,
 request_id uuid NOT NULL,
 UNIQUE(line_user_id, request_id)
);
