-- Prevent ALTER SYSTEM from creating conflicts with config file
-- This runs on every container start to ensure clean state
-- (Only runs on new database initialization, not every restart)

-- Reset any ALTER SYSTEM settings that might conflict with our config file
ALTER SYSTEM RESET ALL;

-- Reload configuration to ensure our postgresql.conf settings take effect
SELECT pg_reload_conf();

-- Log this action for debugging
DO $$
BEGIN
    RAISE NOTICE 'ALTER SYSTEM settings have been reset to ensure config file precedence';
END
$$;