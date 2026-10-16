CREATE TABLE project_documents (
 id uuid PRIMARY KEY,
 site_id uuid NOT NULL REFERENCES sites(id) ON DELETE RESTRICT,
 metadata jsonb NOT NULL,
 latest_version integer NOT NULL DEFAULT 0,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX project_documents_site ON project_documents(site_id);
CREATE TABLE project_document_versions (
 document_id uuid NOT NULL REFERENCES project_documents(id) ON DELETE CASCADE,
 version integer NOT NULL,
 filename text NOT NULL,
 content bytea NOT NULL,
 sha256 text NOT NULL,
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(document_id, version)
);
CREATE TABLE project_document_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 document_id uuid NOT NULL REFERENCES project_documents(id) ON DELETE CASCADE,
 actor_id uuid NOT NULL,
 action text NOT NULL,
 detail jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX project_document_events_document ON project_document_events(document_id, id);
