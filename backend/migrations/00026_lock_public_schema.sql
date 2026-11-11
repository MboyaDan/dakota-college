-- Close the public schema to Supabase's Data API (Supabase security lints
-- 0013 rls_disabled_in_public and 0011 function_search_path_mutable).
--
-- Supabase publishes the public schema through PostgREST to its built-in
-- anon and authenticated roles. This app never uses that API: the Go backend
-- is the only database client (CLAUDE.md), and it connects as the role that
-- owns every table. Row level security with no policies therefore denies
-- the API roles every row while the owner is unaffected (RLS is not FORCEd).
-- Revoking their privileges as well means access needs two mistakes, not one.
-- The anon and authenticated roles exist only on Supabase, so local and CI
-- databases skip that part.
--
-- New tables must enable RLS themselves; TestPublicSchemaIsLocked fails
-- until they do.

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
  t record;
  f record;
BEGIN
  FOR t IN
    SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p')
  LOOP
    EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', t.relname);
  END LOOP;

  -- Our own functions only: those belonging to an extension (citext) are
  -- managed by the extension.
  FOR f IN
    SELECT p.oid::regprocedure AS sig FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
    WHERE n.nspname = 'public'
      AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid = p.oid AND d.deptype = 'e')
  LOOP
    EXECUTE format('ALTER FUNCTION %s SET search_path = public, pg_temp', f.sig);
  END LOOP;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon')
     AND EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
    REVOKE ALL ON ALL TABLES IN SCHEMA public FROM anon, authenticated;
    REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM anon, authenticated;
    REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA public FROM anon, authenticated;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON TABLES FROM anon, authenticated;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON SEQUENCES FROM anon, authenticated;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE EXECUTE ON FUNCTIONS FROM anon, authenticated;
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- Privileges revoked from the Supabase API roles are not restored: granting
-- them back would reopen the schema.
-- +goose StatementBegin
DO $$
DECLARE
  t record;
  f record;
BEGIN
  FOR t IN
    SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p')
  LOOP
    EXECUTE format('ALTER TABLE public.%I DISABLE ROW LEVEL SECURITY', t.relname);
  END LOOP;

  FOR f IN
    SELECT p.oid::regprocedure AS sig FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
    WHERE n.nspname = 'public'
      AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid = p.oid AND d.deptype = 'e')
  LOOP
    EXECUTE format('ALTER FUNCTION %s RESET search_path', f.sig);
  END LOOP;
END
$$;
-- +goose StatementEnd
