# Runbook

What to do for routine changes and when something goes wrong on staging.
Setup from nothing is in [`deployment.md`](deployment.md). Production will get
its own copy of these steps at go-live (Stage 20).

## Deploy

- **API:** merging to `main` with changes under `backend/` deploys on Render
  once GitHub checks pass (`autoDeployTrigger: checksPass`).
- **Site:** every merge to `main` with changes under `frontend/` builds on
  Vercel.
- **Order when both change:** the API must be live first (D-204). If a pull
  request needs a migration, run it before merging (next section).
- After a deploy: `/api/v1/ready` answers ready, the home page loads, staff
  sign-in works.

## Run migrations

Never automatic. A pull request that adds a file under `backend/migrations`
says so in its "Migration and deploy notes".

```sh
cd backend
export APP_ENV=staging
read -rs DATABASE_URL && export DATABASE_URL   # session pooler URL
go run ./cmd/migrate status                     # what is pending
go run ./cmd/migrate up
unset DATABASE_URL
```

Run it **before** merging the pull request: our migrations add things the old
code ignores, so the running API keeps working. If a migration ever removes
or renames something, the pull request must say how to sequence it.

Before a risky migration, take a backup first (Actions → Database backup →
Run workflow).

Production (later): `APP_ENV=production go run ./cmd/migrate -confirm-production up`.

## Roll back

- **API:** Render → `dakota-api-staging` → Events → pick the last good deploy →
  Rollback. Then revert the commit on `main`, so the next deploy does not
  bring it back.
- **Site:** Vercel → Deployments → last good one → Instant Rollback. Revert
  on `main` as above.
- **Migration:** `go run ./cmd/migrate down` rolls back one step. Only if the
  down step loses no data; otherwise restore from a backup into a new
  database and compare first.

## Restore a backup

Backups are nightly encrypted dumps of the `public` schema, kept 7 days as
workflow artifacts (Actions → Database backup → a run → Artifacts).

**Check a backup (do this monthly and after setup):**

```sh
gh run download <run-id> --dir ~/Downloads/dakota-backup
backend/scripts/restore-check.sh ~/Downloads/dakota-backup/*/*.dump.age ~/dakota-staging-backup.key
```

It restores into a throwaway local container, checks that migrations are
current, and prints row counts. Compare the counts with staging.

**Restore for real** (the staging database is lost or corrupted):

1. Stop writes: Render → Suspend the API. The public site keeps rendering from
   its cached copy.
2. Restore into the Supabase database. The dump recreates the `public`
   schema with its `citext` extension, so empty it first with
   `DROP SCHEMA public CASCADE;` (SQL editor). This deletes what is there:
   be sure you are connected to the right project.

   ```sh
   age --decrypt --identity ~/dakota-staging-backup.key backup.dump.age > /tmp/restore.dump
   pg_restore --no-owner --no-privileges --exit-on-error -d "$DATABASE_URL" /tmp/restore.dump
   rm /tmp/restore.dump
   ```

   Use a `pg_restore` of version 17 or newer (`brew install postgresql@17`).

3. `go run ./cmd/migrate status` shows nothing pending.
4. If the project changed, update `DATABASE_URL` on Render and the
   `BACKUP_DATABASE_URL` secret. Resume the API.
5. Note what was lost: everything after the dump time. Files in buckets are
   not in the dump.

## Rotate a secret

| Secret             | How                                                               | Care                                                              |
| ------------------ | ----------------------------------------------------------------- | ----------------------------------------------------------------- |
| `CLIENT_IP_SECRET` | New value on Render and Vercel, redeploy both                     | Until both match, every visitor shares one address in rate limits |
| `SMTP_PASSWORD`    | New SMTP key in Brevo, set on Render, delete the old key          | Emails fail and retry until the new key is in place               |
| `S3_SECRET_KEY`    | New key pair in Supabase, set on Render, delete the old pair      |                                                                   |
| Database password  | Reset in Supabase, update Render and `BACKUP_DATABASE_URL`        | API is down until Render has it                                   |
| `MESSAGE_KEY`      | Only when no messages wait or failed in the last 7 days (R-28)    | Waiting messages become unreadable                                |
| `TOTP_KEY`         | Avoid. Everyone with two-step sign-in must set it up again (R-29) |                                                                   |

If a secret may have leaked, rotate it at once and check the audit log for
sign-ins you do not recognise.

## When something is wrong

| Symptom                                                | Likely cause                                         | What to do                                                                            |
| ------------------------------------------------------ | ---------------------------------------------------- | ------------------------------------------------------------------------------------- |
| First page load takes about a minute                   | Render free API was asleep                           | Expected on free; the uptime monitor should prevent it. Check UptimeRobot is running. |
| Site shows old content                                 | Snapshot cache, refreshes every 5 minutes (D-74)     | Wait; if longer, check the API is up.                                                 |
| Pages show empty states after a deploy                 | Build ran while the API was down: empty snapshot     | Redeploy on Vercel once `/api/v1/ready` answers.                                      |
| `/api/v1/ready` fails, database error                  | Supabase paused or password changed                  | Supabase dashboard → Restore project; check `DATABASE_URL`.                           |
| Uploads fail; log says `storage buckets not reachable` | Bucket names, S3 keys or endpoint wrong              | Check against deployment.md, step 1.                                                  |
| Emails not arriving                                    | Brevo key, daily limit (300), or spam folder         | Staff portal message log shows status; Render logs show SMTP errors.                  |
| Everyone locked out by rate limits                     | `CLIENT_IP_SECRET` differs between Render and Vercel | Set the same value on both.                                                           |
| Render stopped deploying                               | CI did not pass, or Actions minutes ran out          | GitHub → Actions; Settings → Billing.                                                 |
| Render service suspended                               | 750 free hours used up                               | Waits for next month, or upgrade.                                                     |

Logs: Render → service → Logs (JSON, one line per request with request ID);
Vercel → project → Logs. A request ID from an error page matches a Render log
line.

## Contacts and access

| What                                                 | Who holds it                                   |
| ---------------------------------------------------- | ---------------------------------------------- |
| GitHub, Render, Vercel, Supabase, Brevo, UptimeRobot | Developer (Dan Roylex)                         |
| Backup private key                                   | Developer, plus a copy in the password manager |
| Domain and DNS (later)                               | The college, with the developer as admin       |
