-- Fictional metadata created for Infinitune development and demos.
-- This file intentionally contains no third-party audio, lyrics, or artwork.

INSERT INTO catalog.artists (id, name, description, image_url)
VALUES
    ('10000000-0000-4000-8000-000000000001', 'Lumen Harbor', 'Fictional electronic project created for Infinitune demos.', ''),
    ('10000000-0000-4000-8000-000000000002', 'Quiet Vector', 'Fictional producer and composer created for Infinitune demos.', ''),
    ('10000000-0000-4000-8000-000000000003', 'Cedar & Static', 'Fictional indie duo created for Infinitune demos.', '')
ON CONFLICT DO NOTHING;

INSERT INTO catalog.genres (id, name, slug)
VALUES
    ('60000000-0000-4000-8000-000000000001', 'Ambient', 'ambient'),
    ('60000000-0000-4000-8000-000000000002', 'Electronic', 'electronic'),
    ('60000000-0000-4000-8000-000000000003', 'Indie', 'indie'),
    ('60000000-0000-4000-8000-000000000004', 'Synthwave', 'synthwave')
ON CONFLICT DO NOTHING;

INSERT INTO catalog.moods (id, name, slug)
VALUES
    ('70000000-0000-4000-8000-000000000001', 'Focused', 'focused'),
    ('70000000-0000-4000-8000-000000000002', 'Hopeful', 'hopeful'),
    ('70000000-0000-4000-8000-000000000003', 'Nocturnal', 'nocturnal'),
    ('70000000-0000-4000-8000-000000000004', 'Energetic', 'energetic')
ON CONFLICT DO NOTHING;

INSERT INTO catalog.albums (id, title, album_type, release_date, cover_url)
VALUES
    ('20000000-0000-4000-8000-000000000001', 'Borrowed Constellations', 'album', '2025-03-14', ''),
    ('20000000-0000-4000-8000-000000000002', 'Dawn Protocols', 'ep', '2026-06-05', '')
ON CONFLICT DO NOTHING;

WITH seed(album_id, artist_id, position) AS (
VALUES
    ('20000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', 1),
    ('20000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000002', 1),
    ('20000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000001', 2)
)
INSERT INTO catalog.album_artists (album_id, artist_id, position)
SELECT seed.album_id::uuid, seed.artist_id::uuid, seed.position
FROM seed
JOIN catalog.albums ON albums.id = seed.album_id::uuid
JOIN catalog.artists ON artists.id = seed.artist_id::uuid
ON CONFLICT DO NOTHING;

INSERT INTO catalog.tracks (
    id, album_id, title, duration_ms, track_number, disc_number, explicit,
    release_date, media_asset_id, content_id
)
VALUES
    (
        '30000000-0000-4000-8000-000000000001', '20000000-0000-4000-8000-000000000001',
        'First Light Engine', 6000, 1, 1, false, '2025-03-14',
        '40000000-0000-4000-8000-000000000001', '50000000-0000-4000-8000-000000000001'
    ),
    (
        '30000000-0000-4000-8000-000000000002', '20000000-0000-4000-8000-000000000001',
        'Paper Moons', 6000, 2, 1, false, '2025-03-14',
        '40000000-0000-4000-8000-000000000002', '50000000-0000-4000-8000-000000000002'
    ),
    (
        '30000000-0000-4000-8000-000000000003', '20000000-0000-4000-8000-000000000001',
        'Harbor Without Maps', 6000, 3, 1, false, '2025-03-14',
        '40000000-0000-4000-8000-000000000003', '50000000-0000-4000-8000-000000000003'
    ),
    (
        '30000000-0000-4000-8000-000000000004', '20000000-0000-4000-8000-000000000002',
        'Neon Rain Index', 6000, 1, 1, true, '2026-06-05',
        '40000000-0000-4000-8000-000000000004', '50000000-0000-4000-8000-000000000004'
    ),
    (
        '30000000-0000-4000-8000-000000000005', '20000000-0000-4000-8000-000000000002',
        'Parallel Sunrise', 6000, 2, 1, false, '2026-06-05',
        '40000000-0000-4000-8000-000000000005', '50000000-0000-4000-8000-000000000005'
    ),
    (
        '30000000-0000-4000-8000-000000000006', NULL,
        'Cedar Signal', 6000, 1, 1, false, '2026-09-18',
        '40000000-0000-4000-8000-000000000006', '50000000-0000-4000-8000-000000000006'
    )
ON CONFLICT DO NOTHING;

WITH seed(track_id, artist_id, artist_role, position) AS (
VALUES
    ('30000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', 'primary', 1),
    ('30000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000002', 'producer', 1),
    ('30000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000001', 'primary', 1),
    ('30000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000002', 'featured', 1),
    ('30000000-0000-4000-8000-000000000003', '10000000-0000-4000-8000-000000000001', 'primary', 1),
    ('30000000-0000-4000-8000-000000000004', '10000000-0000-4000-8000-000000000002', 'primary', 1),
    ('30000000-0000-4000-8000-000000000004', '10000000-0000-4000-8000-000000000001', 'producer', 1),
    ('30000000-0000-4000-8000-000000000005', '10000000-0000-4000-8000-000000000002', 'primary', 1),
    ('30000000-0000-4000-8000-000000000005', '10000000-0000-4000-8000-000000000001', 'featured', 1),
    ('30000000-0000-4000-8000-000000000006', '10000000-0000-4000-8000-000000000003', 'primary', 1)
)
INSERT INTO catalog.track_artists (track_id, artist_id, artist_role, position)
SELECT seed.track_id::uuid, seed.artist_id::uuid,
       seed.artist_role::catalog.artist_role, seed.position
FROM seed
JOIN catalog.tracks ON tracks.id = seed.track_id::uuid
JOIN catalog.artists ON artists.id = seed.artist_id::uuid
ON CONFLICT DO NOTHING;

WITH seed(track_id, genre_name, genre_slug) AS (
VALUES
    ('30000000-0000-4000-8000-000000000001', 'Electronic', 'electronic'),
    ('30000000-0000-4000-8000-000000000001', 'Synthwave', 'synthwave'),
    ('30000000-0000-4000-8000-000000000002', 'Ambient', 'ambient'),
    ('30000000-0000-4000-8000-000000000002', 'Electronic', 'electronic'),
    ('30000000-0000-4000-8000-000000000003', 'Ambient', 'ambient'),
    ('30000000-0000-4000-8000-000000000003', 'Indie', 'indie'),
    ('30000000-0000-4000-8000-000000000004', 'Electronic', 'electronic'),
    ('30000000-0000-4000-8000-000000000004', 'Synthwave', 'synthwave'),
    ('30000000-0000-4000-8000-000000000005', 'Electronic', 'electronic'),
    ('30000000-0000-4000-8000-000000000005', 'Synthwave', 'synthwave'),
    ('30000000-0000-4000-8000-000000000006', 'Ambient', 'ambient'),
    ('30000000-0000-4000-8000-000000000006', 'Indie', 'indie')
)
INSERT INTO catalog.track_genres (track_id, genre_id)
SELECT seed.track_id::uuid, resolved.id
FROM seed
JOIN catalog.tracks ON tracks.id = seed.track_id::uuid
JOIN LATERAL (
    SELECT id
    FROM catalog.genres
    WHERE lower(slug) = lower(seed.genre_slug)
       OR lower(name) = lower(seed.genre_name)
    ORDER BY (lower(slug) = lower(seed.genre_slug)) DESC, id
    LIMIT 1
) AS resolved ON true
ON CONFLICT DO NOTHING;

WITH seed(track_id, mood_name, mood_slug, weight) AS (
VALUES
    ('30000000-0000-4000-8000-000000000001', 'Hopeful', 'hopeful', 0.900),
    ('30000000-0000-4000-8000-000000000001', 'Energetic', 'energetic', 0.700),
    ('30000000-0000-4000-8000-000000000002', 'Focused', 'focused', 0.800),
    ('30000000-0000-4000-8000-000000000002', 'Nocturnal', 'nocturnal', 0.600),
    ('30000000-0000-4000-8000-000000000003', 'Focused', 'focused', 0.700),
    ('30000000-0000-4000-8000-000000000003', 'Hopeful', 'hopeful', 0.600),
    ('30000000-0000-4000-8000-000000000004', 'Nocturnal', 'nocturnal', 0.900),
    ('30000000-0000-4000-8000-000000000004', 'Energetic', 'energetic', 0.900),
    ('30000000-0000-4000-8000-000000000005', 'Hopeful', 'hopeful', 0.800),
    ('30000000-0000-4000-8000-000000000005', 'Energetic', 'energetic', 0.700),
    ('30000000-0000-4000-8000-000000000006', 'Focused', 'focused', 0.700),
    ('30000000-0000-4000-8000-000000000006', 'Hopeful', 'hopeful', 0.500)
)
INSERT INTO catalog.track_moods (track_id, mood_id, weight)
SELECT seed.track_id::uuid, resolved.id, seed.weight
FROM seed
JOIN catalog.tracks ON tracks.id = seed.track_id::uuid
JOIN LATERAL (
    SELECT id
    FROM catalog.moods
    WHERE lower(slug) = lower(seed.mood_slug)
       OR lower(name) = lower(seed.mood_name)
    ORDER BY (lower(slug) = lower(seed.mood_slug)) DESC, id
    LIMIT 1
) AS resolved ON true
ON CONFLICT DO NOTHING;
