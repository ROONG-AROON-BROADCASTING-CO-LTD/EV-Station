ALTER TABLE site_images
  DROP CONSTRAINT IF EXISTS site_images_mime_type_check;

ALTER TABLE site_images
  ADD CONSTRAINT site_images_mime_type_check
  CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/webp'));
