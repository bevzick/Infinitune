DELETE FROM auth.roles
WHERE name IN ('listener', 'admin', 'moderator', 'creator');
