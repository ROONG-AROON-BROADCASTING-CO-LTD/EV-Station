-- Legacy webhook discovery did not establish staff consent. Existing rows
-- remain unapproved until an owner initiates a signed, one-time pairing.
ALTER TABLE line_notification_recipient ADD COLUMN approved_at timestamptz;
