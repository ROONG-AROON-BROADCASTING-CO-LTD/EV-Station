CREATE TABLE line_notification_recipient (
  id boolean PRIMARY KEY DEFAULT true CHECK (id),
  recipient_id text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
