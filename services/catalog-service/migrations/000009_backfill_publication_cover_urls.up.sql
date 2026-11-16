UPDATE catalog.albums AS album
SET cover_url = '/v1/media/covers/' || publication.source_release_id::text,
    updated_at = now()
FROM catalog.publications AS publication
WHERE publication.album_id = album.id
  AND btrim(album.cover_url) = '';
