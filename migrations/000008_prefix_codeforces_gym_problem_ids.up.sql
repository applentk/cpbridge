-- Preserve the Gym source context in external IDs so statement retrieval uses
-- /gym/{contest}/problem/{index}, not the normal problemset URL.
UPDATE problems AS legacy
SET external_id = 'gym/' || legacy.external_id
WHERE legacy.platform = 'CODEFORCES'
  AND COALESCE(legacy.metadata->>'gym', 'false') = 'true'
  AND legacy.external_id !~ '^gym/'
  AND NOT EXISTS (
    SELECT 1
    FROM problems AS current
    WHERE current.platform = legacy.platform
      AND current.external_id = 'gym/' || legacy.external_id
  );
