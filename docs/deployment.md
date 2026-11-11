# Deployment

How the staging environment is put together and how to set it up from
nothing. Day-to-day operations (migrations, rollback, restore, secret
rotation) are in [`runbook.md`](runbook.md).

**Free tiers are for staging only, with no real personal data** (brief 12.2,
A-05). The list of what must change before real data is at the end.

## Shape

```
Browser ──HTTPS──► Vercel (Next.js, fra1)
                     │  /api/* rewrite, adds visitor IP + shared secret (D-195)
                     ▼
                   Render (Go API, Docker, Frankfurt, free)
                     │                         │
                     ▼                         ▼
          Supabase PostgreSQL         Supabase Storage (S3 API)
          (session pooler, IPv4)      dakota-public / dakota-private
                     │
                     ▼
          Brevo SMTP (email)       UptimeRobot (uptime)    GitHub Actions (CI, nightly backup)
```

- The browser only talks to the Vercel origin. Only the API holds database
  and storage credentials. Supabase is used only as PostgreSQL and S3 storage.
- Public pages are cached and keep a bundled copy of the content, so the site
  still renders while the free API is asleep (ADR-03).
- Region: everything in Frankfurt (Render `frankfurt`, Supabase
  `eu-central-1`, Vercel functions `fra1`). No provider has an African region.

## What the free tiers mean in practice

| Service                       | Limit that matters                                                                                                       | Effect on staging                                                                                                                                                                                                                      |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Render Free                   | Sleeps after 15 minutes without traffic; waking takes about a minute; 750 instance hours a month per workspace; no shell | The uptime check keeps it awake (one service uses at most 744 hours a month). Queued email and the daily retention run only happen while it is awake. Migrations run from your laptop.                                                 |
| Supabase Free                 | Pauses after a week of inactivity; no automatic backups; 500 MB database, 1 GB storage                                   | The uptime check reaches the database, so it does not pause. Backups come from our nightly GitHub Action.                                                                                                                              |
| Vercel Hobby                  | Non-commercial use only (brief 12.2)                                                                                     | Fine for a private staging link the developer uses. **Before the college uses it, move to Vercel Pro or Cloudflare.** No password protection on the production URL. Search engines are kept out by `noindex` (`SITE_INDEXABLE` unset). |
| Brevo Free                    | 300 emails a day; sender is one verified address                                                                         | Without a domain, mail comes from a personal address and may land in spam.                                                                                                                                                             |
| GitHub Actions (private repo) | 2,000 minutes a month                                                                                                    | CI uses about 15 job-minutes a run. At October's pace (60 runs) that is near the limit. When minutes run out, CI stops, and Render (deploying on `checksPass`) stops deploying.                                                        |

## Accounts to create (once)

Turn on two-factor sign-in for every one of them (brief 12.4).

1. **GitHub**: make the repository private:
   `gh repo edit MboyaDan/dakota-college --visibility private --accept-visibility-change-consequences`
2. **Supabase** (supabase.com): new project `dakota-staging`, region
   **Central EU (Frankfurt)**, a strong database password kept in your
   password manager.
3. **Render** (render.com), signed in with GitHub.
4. **Vercel** (vercel.com), Hobby, signed in with GitHub.
5. **Brevo** (brevo.com), free plan.
6. **UptimeRobot** (uptimerobot.com), free plan.

## Secrets to generate (once per environment)

Run each command once and paste the output straight into the host's secret
field. Never commit them, never reuse local values, and never reuse one value
for two settings.

| Setting            | Command                                     | Where it goes                                                                                                                              |
| ------------------ | ------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `MESSAGE_KEY`      | `openssl rand -base64 32`                   | Render                                                                                                                                     |
| `TOTP_KEY`         | `openssl rand -base64 32`                   | Render                                                                                                                                     |
| `CLIENT_IP_SECRET` | `openssl rand -hex 32`                      | Render **and** Vercel (same value)                                                                                                         |
| Backup key pair    | `age-keygen -o ~/dakota-staging-backup.key` | Public key (`age1…`, printed) to GitHub variable `BACKUP_AGE_RECIPIENT`; the key file stays with you, plus a copy in your password manager |

Losing `MESSAGE_KEY` makes waiting emails unreadable (R-28). Losing
`TOTP_KEY` makes everyone's authenticator setup unreadable (R-29). Losing the
backup key makes every backup useless.

## Step by step

Order matters: database, then API, then site (D-204).

### 1. Supabase

1. **Database URL.** Project → Connect → **Session pooler** (port 5432, IPv4).
   Use this, not the direct connection, which is IPv6 only on the free plan,
   and not the transaction pooler (6543), which breaks prepared statements
   (R-14). Add `sslmode=require` and a small pool:

   ```
   postgresql://postgres.<ref>:<password>@aws-0-eu-central-1.pooler.supabase.com:5432/postgres?sslmode=require&pool_max_conns=5
   ```

   (Copy the host exactly as Supabase shows it; the `aws-0` part can differ.)

2. **Buckets.** Storage → New bucket:
   - `dakota-public`: **Public** on. Website images only.
   - `dakota-private`: Public off. Applicant documents, letters.
     Leave Row Level Security policies empty: the API uses S3 keys that bypass
     them, and nothing else may reach the buckets.
3. **S3 keys.** Project Settings → Storage → S3 Connection → New access key.
   Note the endpoint (`https://<ref>.storage.supabase.co/storage/v1/s3`) and
   region (`eu-central-1`). These keys give full storage access; they go only
   to Render.
4. **Data API off.** The site never uses Supabase's Data API (PostgREST); only
   the Go API talks to the database. Project Settings → Data API: turn
   **Enable Data API** off (or remove `public` from the exposed schemas).
   Migration 00026 also locks the schema on its own (row level security with
   no policies on every table, no privileges for `anon` and `authenticated`,
   D-227), so the lock holds even if the API is turned back on. After
   migrating, Advisors → Security Advisor should show no errors; the one
   warning left is `citext` in `public` (accepted, D-227).

### 2. Migrations and the first Super Admin (from your laptop)

The free API has no shell, and migrations never run automatically.

```sh
cd backend
export APP_ENV=staging
read -rs DATABASE_URL && export DATABASE_URL   # paste the session pooler URL
go run ./cmd/migrate status
go run ./cmd/migrate up
read -rs SUPER_ADMIN_PASSWORD && export SUPER_ADMIN_PASSWORD  # 12+ characters
SUPER_ADMIN_EMAIL=you@example.com SUPER_ADMIN_NAME="Your Name" go run ./cmd/setup
unset DATABASE_URL SUPER_ADMIN_PASSWORD
```

Do not run `seed`: it refuses Supabase hosts and `APP_ENV=staging` anyway
(D-39). Staging content is entered through the staff portal.

### 3. Brevo

1. Senders → add your sender address, click the confirmation link.
2. SMTP & API → SMTP → generate an SMTP key. Note the SMTP login (looks like
   `xxxx@smtp-brevo.com`) and the key.

### 4. Render (API)

1. New → Blueprint → choose this repository. Render reads `render.yaml`.
2. Fill in the values it asks for (table below). `APP_BASE_URL` and
   `ALLOWED_ORIGINS` need the Vercel URL. Choose the Vercel project name now
   (for example `dakota-staging` gives `https://dakota-staging.vercel.app`),
   then create the Vercel project with that name in step 5.
3. Wait for the first deploy, then open
   `https://dakota-api-staging.onrender.com/api/v1/ready`. It should answer
   ready. The log should say `storage buckets reachable`.

### 5. Vercel (site)

1. Add New → Project → import this repository.
2. **Root Directory: `frontend`.** Framework: Next.js (from `vercel.json`).
3. Environment variables (Production), from the table below.
4. Deploy. The build log should say `snapshot: N schools, M programmes from
https://dakota-api-staging.onrender.com`. If it says `writing an empty
snapshot`, the API was not ready: redeploy once it answers.
5. Settings → Git → Ignored Build Step → "Only build if there are changes in
   a folder": `frontend` (saves builds on backend-only changes).

### 6. Uptime and backups

1. UptimeRobot → New monitor → HTTP(s),
   `https://dakota-api-staging.onrender.com/api/v1/ready`, every 5 minutes,
   alert to your email. A second monitor for the Vercel home page.
2. GitHub → Settings → Secrets and variables → Actions:
   - secret `BACKUP_DATABASE_URL`: the session pooler URL, without
     `pool_max_conns`
   - variable `BACKUP_AGE_RECIPIENT`: the `age1…` public key
3. Actions → Database backup → Run workflow. Then download the artifact and
   run the restore check (runbook, "Restore a backup").

## Environment variables

### Render (API)

| Name                                    | Value                                                               | Secret |
| --------------------------------------- | ------------------------------------------------------------------- | ------ |
| `APP_ENV`                               | `staging` (in `render.yaml`)                                        |        |
| `PORT`, `HTTP_ADDR`                     | `8080`, `:8080` (in `render.yaml`)                                  |        |
| `DATABASE_URL`                          | Supabase session pooler URL with `sslmode=require&pool_max_conns=5` | yes    |
| `APP_BASE_URL`                          | `https://<vercel-project>.vercel.app`                               |        |
| `ALLOWED_ORIGINS`                       | same as `APP_BASE_URL`                                              |        |
| `CLIENT_IP_SECRET`                      | `openssl rand -hex 32`, same as Vercel                              | yes    |
| `MESSAGE_KEY`                           | `openssl rand -base64 32`                                           | yes    |
| `TOTP_KEY`                              | `openssl rand -base64 32`                                           | yes    |
| `SESSION_COOKIE_SECURE`                 | `true` (in `render.yaml`)                                           |        |
| `SMTP_HOST`, `SMTP_PORT`                | `smtp-relay.brevo.com`, `587` (in `render.yaml`)                    |        |
| `SMTP_USERNAME`                         | Brevo SMTP login                                                    | yes    |
| `SMTP_PASSWORD`                         | Brevo SMTP key                                                      | yes    |
| `SMTP_FROM`                             | the verified sender address, bare (D-50)                            |        |
| `S3_ENDPOINT`                           | `https://<ref>.storage.supabase.co/storage/v1/s3`                   |        |
| `S3_REGION`                             | `eu-central-1` (in `render.yaml`)                                   |        |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY`        | Supabase S3 key pair                                                | yes    |
| `S3_PUBLIC_BUCKET`, `S3_PRIVATE_BUCKET` | `dakota-public`, `dakota-private` (in `render.yaml`)                |        |
| `S3_PUBLIC_BASE_URL`                    | `https://<ref>.supabase.co/storage/v1/object/public/dakota-public`  |        |
| `RETENTION_MODE`                        | `report` (in `render.yaml`; `apply` only after DG-06)               |        |
| `CLAMAV_ADDR`                           | not set: no virus scanner on staging (D-221)                        |        |
| `MPESA_*`, `KCB_*`                      | not set: payment integrations off on staging                        |        |

### Vercel (site)

| Name                    | Value                                                         | Secret |
| ----------------------- | ------------------------------------------------------------- | ------ |
| `API_ORIGIN`            | `https://dakota-api-staging.onrender.com`                     |        |
| `CONTENT_IMAGE_ORIGIN`  | `https://<ref>.supabase.co`                                   |        |
| `CLIENT_IP_HEADER`      | `x-real-ip` (Vercel overwrites it with the visitor's address) |        |
| `CLIENT_IP_SECRET`      | same value as Render                                          | yes    |
| `SITE_URL`              | `https://<vercel-project>.vercel.app`                         |        |
| `SNAPSHOT_WAIT_SECONDS` | `150`                                                         |        |
| `SITE_INDEXABLE`        | not set (staging is not indexed)                              |        |

### GitHub Actions

| Name                   | Kind     | Value                       |
| ---------------------- | -------- | --------------------------- |
| `BACKUP_DATABASE_URL`  | secret   | Supabase session pooler URL |
| `BACKUP_AGE_RECIPIENT` | variable | age public key              |

## Domain later (DG-01)

When the domain is bought (from a KENIC-accredited registrar for `.co.ke` or
`.ac.ke`, brief 12.3):

1. Point its nameservers to Cloudflare (free plan). HTTPS is issued by Vercel
   and Render for custom domains.
2. Site: add the domain in Vercel and create the DNS record Vercel shows
   (DNS only, grey cloud, so Vercel's `x-real-ip` stays the visitor's address;
   if you turn the Cloudflare proxy on, change `CLIENT_IP_HEADER` to
   `cf-connecting-ip`).
3. API: optional `api.` subdomain on Render. The browser never calls it
   directly, so the `onrender.com` address can stay.
4. Change `APP_BASE_URL`, `ALLOWED_ORIGINS` (Render) and `SITE_URL` (Vercel)
   to the new origin, and redeploy both.
5. Brevo: authenticate the domain (SPF, DKIM, DMARC records in Cloudflare)
   and switch `SMTP_FROM` to an address on it, for example
   `no-reply@<domain>`.
6. `CONTENT_IMAGE_ORIGIN` stays the Supabase origin.

## Before real data (production)

None of this is optional (brief 12.2, A-05, DG-04):

- [ ] Hosting decision recorded (DG-04): managed stack or VPS.
- [ ] Supabase **Pro** (daily backups kept 7 days, no pausing). Keep our nightly
      encrypted dump too, and run a restore test.
- [ ] Render **paid** instance (no sleeping; the email queue and daily jobs
      need the process running).
- [ ] Vercel **Pro** or Cloudflare (Hobby is non-commercial).
- [ ] A separate production project on each service, with its own secrets.
      Never point production at the staging database or buckets.
- [ ] Domain with SPF, DKIM and DMARC on the sending address.
- [ ] Repository private, two-factor on every account, access limited to the
      people who need it.
- [ ] Virus scanning decision for uploads (D-221): a ClamAV instance
      (about 1 GB of memory) or a documented acceptance of the risk.
- [ ] Storage backups: Supabase backups cover the database, not files in
      buckets. Decide how applicant documents are backed up.
- [ ] Error monitoring beyond host logs and uptime alerts (D-222).
- [ ] Privacy notice approved and `RETENTION_MODE=apply` (DG-06, R-30).
- [ ] Migrations run with `APP_ENV=production go run ./cmd/migrate -confirm-production up`.
