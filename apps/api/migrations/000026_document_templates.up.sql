CREATE TABLE document_templates (
 id uuid PRIMARY KEY,
 title text NOT NULL,
 filename text NOT NULL,
 content bytea NOT NULL,
 sha256 text NOT NULL,
 uploaded_by uuid,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE document_template_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 template_id uuid NOT NULL REFERENCES document_templates(id) ON DELETE RESTRICT,
 actor_id uuid,
 action text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX document_template_events_template ON document_template_events(template_id,id);
