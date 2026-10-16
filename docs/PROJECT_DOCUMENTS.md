# Project documents and contracts

Available at `/documents` and `/sites/:id/documents`. A public preparation
checklist also appears on the investment page and site submission form.

Contracts use separate PostgreSQL tables introduced by migration 000025.
They do not enter `site_images`, analysis evidence or Gemini prompts. Files
are authenticated PDFs (10 MiB maximum each); all retained versions together
may total 100 MiB per project. Originals and SHA-256 fingerprints are retained.
The existing database backup must include these tables and their binary data.

Owners/admins access all projects; sales access assigned projects. Only staff
can upload, change metadata, confirm signature completeness or release files.
Customers access only their assigned projects and explicitly released latest
versions. Earlier versions, private documents and audit history are denied
server-side, including direct file URLs. Release includes the full PDF and its
summary: the system does not redact bank details or signatures automatically.

New uploads start pending review. A replacement version revokes customer
release and signature confirmation. Replaying an identical latest upload does
not add another version. Initial browser uploads use author/project-scoped
request IDs to prevent duplicate records after a lost response. Metadata saves
carry an expected version; a stale reviewer receives 409 instead of releasing
an unseen replacement file. Metadata edits, uploads, previews and downloads
are audited. The UI shows the latest 200 events with actors and metadata changes.
Underlying event records are retained without automatic pruning.

Contract dates are entered explicitly. Signed/active statuses require a manual
signature-completeness confirmation. Active status also requires a current
start/end period. This is document workflow, not legal validation, OCR, an
electronic-signature service or automated expiry notifications. Never infer
an end date from an unfilled scan or a stated five-year term alone.

Projects with retained documents cannot be deleted through the site API (409).
Use document statuses to archive the record. There is no document-delete API.

For release, back up PostgreSQL, apply migrations 000025–000026, then replace API/web
images. Roll back application images if necessary while leaving the added
tables intact; the down migration deliberately drops the document records
and must not be used as a production rollback after real uploads.

Verification: isolated API/PostgreSQL browser flows cover upload, retry,
version retention, stale-review conflicts, customer release/revocation,
unauthorized direct access, audit reads and project-deletion protection on
desktop/mobile Chromium. Native mobile PDF viewers and physical Safari need
separate device checks.

## Shared examples

The operator confirmed the Orchid PDF is an example for all projects, not a
contract to attach to the Phitsanulok site. Migration 000026 adds a separate
shared template library, shown to staff on every project-document page.
Administrators upload reference PDFs; sales may read/download them. Customer
access is denied even through direct API/file URLs. Template reads and uploads
are audited in `document_template_events`. The library is capped at 100 MiB.

The supplied scan is retained unchanged as an internal reference, including its
original counterparty details. It is not a blank/redacted legal template and is
not automatically attached to projects or sent to customers. Staff must prepare
and review a project-specific copy before using the customer-release control.
The system does not generate new contract text or reproduce signatures.
Existing sites are not renamed or created from the example scan.

Released 16 October 2026 with schema version 26, no dirty migration state.
The original sample was imported once into the private library and its SHA-256
was verified against the supplied local PDF (4,226,406 bytes). Import is marked
as an operator action, not a customer submission. Public unauthenticated access
to both the template list and original-file API returns 401.
