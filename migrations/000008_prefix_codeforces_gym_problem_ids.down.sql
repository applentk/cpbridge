-- Revert only when doing so cannot collide with an existing legacy ID.
UPDATE problems AS prefixed
SET external_id = substring(prefixed.external_id FROM 5)
WHERE prefixed.platform = 'CODEFORCES'
  AND COALESCE(prefixed.metadata->>'gym', 'false') = 'true'
  AND prefixed.external_id ~ '^gym/[0-9]+/.+$'
  AND NOT EXISTS (
    SELECT 1
    FROM problems AS legacy
    WHERE legacy.platform = prefixed.platform
      AND legacy.external_id = substring(prefixed.external_id FROM 5)
  );
