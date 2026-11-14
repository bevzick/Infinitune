ALTER TYPE content.release_status
    ADD VALUE IF NOT EXISTS 'archived' AFTER 'published';
