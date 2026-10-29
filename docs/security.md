# Security and privacy review

Written threat review for the Dakota Training College platform (brief section 13, Stage 17). It records what the platform protects, from whom, how, the evidence for each control, what this review found and fixed, and what must still happen before real people's data goes in.

- **Last reviewed:** 2026-10-29, Stage 17a, against `main` after #43 plus the Stage 17a branch.
- **Covers:** the Go API, the Next.js site and the local Compose stack. Hosting, backups and DNS are reviewed in Stage 19 and are listed below as open items.
- **Still to come in Stage 17:** retention jobs, data subject request tools and a breach procedure (17c). Two-step sign-in was added in 17b (section 4.2).
- **Rule for this file:** a control is listed as in place only if a test or a command shows it. Anything not yet checked says so.

## 1. What we protect

| Asset | Where it lives | Why it matters |
|---|---|---|
| Applicant personal data, many applicants under 18: identity numbers, dates of birth, contacts, guardians, education, uploaded ID and certificates | PostgreSQL; private storage bucket | Data Protection Act 2019; minors need extra care (brief 5.3, R-04) |
| Student records: results, transcripts, fees, payments | PostgreSQL; private bucket (PDFs) | Academic and financial integrity; never edited destructively (NN-05) |
| Staff accounts and their permissions | PostgreSQL (`users`, `roles`, `sessions`) | A staff account can read or change everything above |
| Money records: invoices, receipts, M-Pesa and KCB notifications | PostgreSQL | Fraud and reconciliation (NN-03, NN-10) |
| Secrets: database URL, `MESSAGE_KEY`, `TOTP_KEY`, payment callback secrets, Daraja and Buni keys, `CLIENT_IP_SECRET`, SMTP and S3 credentials | Environment variables only; local `.env` is git-ignored | Any one of them opens a back door |
| The public website's content and reputation | PostgreSQL, public bucket, site cache | Defacement or false claims (NN-01) |

## 2. Who might attack, and how

| Actor | Likely goals | Main routes in |
|---|---|---|
| Anonymous visitor or bot | Guess passwords, flood forms, scrape, deface, lock people out | Public endpoints (sign-in, reset, enquiry, applicant link), file upload, the site |
| Applicant or student | See or change someone else's application, results or fees | Their own session plus guessed IDs |
| Staff member | Go beyond their role (a trainer reading all students, Finance editing a receipt) | Their session; role configuration |
| Someone with a stolen session or a malicious page in another tab | Act as the victim | Cross-site requests, injected script |
| Someone pretending to be Safaricom or KCB | Credit payments that never happened | Payment callback endpoints |
| A compromised dependency | Run code in the API or the browser | npm and Go modules |

## 3. Trust boundaries

```
Browser ──HTTPS──▶ Next.js site (frontend origin)
                      │  pages: CSP, headers (proxy.ts, next.config.ts)
                      │  /api/* rewrite + verified visitor address (D-195)
                      ▼
                   Go API ──▶ PostgreSQL (only database client, NN-02)
                      ├────▶ object storage (private and public buckets)
                      ├────▶ SMTP (queued, encrypted while waiting, D-178)
                      └────▶ ClamAV (when configured)
Safaricom / KCB ──HTTPS──▶ Go API callback paths (secret path, allow list)
```

The browser never talks to the database or storage directly, except for public content images, which load from the public bucket.

## 4. Controls, with evidence

### 4.1 Authorisation (every endpoint)

- **Inventory:** [endpoints.md](endpoints.md) lists all routes and who may call each, generated from the route table; `TestEndpointInventoryIsCurrent` fails if it drifts (D-198). Count at this review: 208 routes (19 public, 149 staff, 16 applicant, 21 student, 3 for any signed-in account).
- **Every protected route refuses the wrong caller:** `TestEveryRouteIsProtected` walks the table. Every non-public route gives 401 without a session and 403 to a user holding every other permission. The public list is reviewed in the test itself (20 routes: two health probes, staff and student sign-in and reset, the two-step sign-in code, site content, news and events, the enquiry form, applicant sign-in links, and the three payment callbacks that answer only on a secret path).
- **Staff routes need a staff account (new, D-196):** a student account with a Super Admin role inserted directly in the database still gets 403 on staff and admin routes, and the API refuses to give roles to non-staff accounts (`TestPermissionRoutesNeedAStaffAccount`).
- **Own records only (object level):** applicant and student routes take the account from the session, never from the request. `TestApplicantRoutesRefuseOtherApplicants` (new) sends every applicant route that takes an application ID with another applicant's session: all are refused and the owner's draft is unchanged. Earlier tests cover the rest: `TestStudentsSeeOnlyTheirOwnData`, `TestTrainersSeeOnlyTheirUnits`, `TestHeadOfSchoolScope`, another student's invoice in `TestFinanceFlow`, another student's transcript in `TestResultsSlipAndTranscripts`, file links bound to one account in `TestDocumentLinks`.
- **Privilege protections:** the last Super Admin cannot be removed, `system.admin` cannot be granted through roles, and roles in use cannot be deleted (`TestSuperAdminAndRoleProtections`, `TestConcurrentSuperAdminRemovalLeavesOne`).

### 4.2 Sign-in and sessions

- Argon2id (19 MiB, 2 passes, 1 lane: the OWASP baseline), passwords 12 to 256 characters, no admin ever sets a staff password (D-47).
- Server-side sessions in an HttpOnly, SameSite=Lax cookie, `__Host-` prefixed and Secure outside local development; 12-hour lifetime and 30-minute idle timeout. Staging and production refuse to start with insecure cookies or http origins (`TestFromLookupErrors`).
- Five wrong passwords lock an account for 15 minutes, stored in the database so a restart does not reset it. A password reset ends every session.
- Unknown accounts take the same path and time as known ones (dummy hash), so answers do not reveal who has an account.
- **Two-step sign-in (Stage 17b, D-206 to D-210):** optional for every staff account, suggested on the Overview to holders of finance, registrar and admin permissions. Authenticator app codes (RFC 6238: 6 digits, 30 seconds, one step either side) or one of ten single-use recovery codes. With it on, a right password only gets a five-minute challenge in an HttpOnly, SameSite=Strict cookie, and the session starts after the code. Each code works once. Wrong codes count towards the same database lockout as wrong passwords, and five wrong codes end a challenge. The secret is stored encrypted with `TOTP_KEY` (AES-256-GCM, bound to the user); recovery codes are stored hashed. Setting up, getting new codes and turning it off need the password again; turning it on signs out the person's other sessions. A password reset does not turn it off. An admin with `users.manage` can reset it for a lost phone, but not their own, and only a Super Admin can reset a Super Admin. `setup reset-two-step EMAIL` on the server covers the case where nobody can sign in. Everything is audited. Evidence: `TestTOTPCodeRFC6238` (RFC test vectors), `TestMatchTOTP`, `TestSecretBoxBindsToUser`, `TestRecoveryCodes`, `TestTwoStepSignIn`, `TestTwoStepWrongCodesLockTheAccount`, `TestTwoStepAdminReset`, `TestTOTPKey`; Playwright `two-step.spec.ts` (set up, sign in with an app code and with a recovery code, turn off, axe at 360 px and desktop).

### 4.3 Cross-site requests

- Every signed-in change needs the session's CSRF token, and the API refuses state-changing requests marked cross-site or from an origin not on `ALLOWED_ORIGINS` (`TestCSRFAndOriginChecks`).

### 4.4 Rate limits

| Endpoint | Limit |
|---|---|
| Staff and student sign-in | 5 failures per account, wrong two-step codes included (database lockout); 5 attempts per email and 100 per visitor address per 15 min; 5 codes per sign-in challenge |
| Password reset (staff and student) | 3 per email and 30 per visitor address per hour; always the same answer |
| Applicant sign-in link and resume | 5 per email and 30 per visitor address per hour |
| Contact form | Honeypot, minimum fill time, at most 2 links, 3 per contact per hour, 60 per hour overall (D-70) |
| Payment callbacks | Refused callbacks throttled per address; allow lists required in production (D-173) |
| Request bodies | 1 MB JSON; uploads 10 MB plus form overhead; callbacks smaller |

**Fixed in this review (D-195, R-23):** behind the Next.js rewrite the API saw every visitor as the frontend server, so the per-address limit was one shared limit. Anyone could send 100 failed sign-ins in 15 minutes and lock every staff member and student out, again and again. The frontend now passes the address its hosting platform reports, with a shared secret, and the API believes it only with that secret. Evidence: `TestTrustedClientIP`; `TestVerifiedVisitorAddressReachesAuditAndLimits` (one address that used up its allowance gets 429 while another visitor still gets a normal answer); `forwardClientIP` unit tests; a live check through the real rewrite (a verified visitor logged and audited as their own address, while a visitor-forged header fell back to the connection address). **Not active until Stage 19 sets `CLIENT_IP_HEADER` and `CLIENT_IP_SECRET`.** The API refuses to start in staging or production without the secret.

Limits are in memory per API instance (D-46). The account lockout is in the database. Running more than one instance would need a shared store.

### 4.5 Browser protections

- **Content-Security-Policy (D-191):** pages that show personal data (staff portal, student portal, application form) get a fresh nonce per request with `'strict-dynamic'`, so injected markup cannot run script. Cached public pages use a fixed policy that still allows inline scripts, so they keep working from cache when the API is asleep. Both block framing, plugins, other-origin connections and `<base>` tricks, and allow images only from the site and the content bucket.
- **Evidence:** `security-headers.spec.ts` checks the headers on public and signed-in pages, that each response gets a different nonce, that the pages still work, and that the staff and student portals run with no policy violations. A control step injects an `<img onerror>` payload on a signed-in page and checks that it is blocked and reported. On the built site, every script on 8 signed-in pages carried the response's nonce (curl count, including pages with an ID in the URL).
- **Other headers (D-192):** HSTS, `nosniff`, frame denial, referrer policy and a permissions policy on pages. The API sends `nosniff`, frame denial, `no-referrer`, same-origin resource policy, `no-store` unless the data is public, and HSTS behind HTTPS (`TestSecurityHeaders`). Files served through links get a sandboxing policy of their own.
- **Open:** HSTS has no `includeSubDomains` or preload until the domain is known (DG-01).

### 4.6 Uploads and files

- Staff images: content sniffed (not trusted from the name or header), 10 MB limit, re-encoded with metadata including GPS removed (`TestProcessAppliesOrientationAndStripsMetadata`).
- Applicant documents: PDF, JPEG or PNG by content, 10 MB each, at most 12, private bucket, links valid 5 minutes for the account they were issued to, token stored hashed, staff links audited (`TestDocumentUploads`, `TestDocumentLinks`).
- Virus scanning: with `CLAMAV_ADDR` set, an infected file is refused and nothing is kept, and a scanner error refuses the upload instead of storing it unchecked (`TestDocumentScanning`). **A real ClamAV now runs:** `TestClamAVLive` caught the EICAR test file and passed a 200 KB clean file locally (2026-10-29), and a CI job repeats it (D-194). Before this review the Compose image had no arm64 build, so the scanner had never run here. Without a scanner, files are stored as `not_scanned` and the API logs a warning.
- **Open:** whether production runs a scanner (it needs about 2 GB of memory) is a hosting decision (Stage 19, DG-04).

### 4.7 Payments

- M-Pesa and KCB callbacks answer only on a secret path, are stored before processing, are processed idempotently and are checked against allow lists in production. An optional ceiling sends large automatic credits to manual confirmation (D-169 to D-176). Manual payments need a finance confirmation (NN-10).
- **Open:** KCB's signature check waits for KCB's key and method (Stage 15b). Until then KCB notifications become pending payments that Finance confirms.

### 4.8 Secrets, logging and audit

- Secrets come only from the environment. `gitleaks` scans every push. Nothing secret is in the frontend: `CLIENT_IP_SECRET` is a server-side setting of the Next.js server (never `NEXT_PUBLIC_`) and never reaches the browser.
- Access logs record the path only, never query strings, and record callback secrets and file tokens as their route pattern (`loggedPath`; M-Pesa redaction test). Request bodies, passwords and tokens are never logged. Queued email text is encrypted at rest and erased once sent (D-178 to D-180).
- The audit log is append-only (database trigger) and records sign-ins and failures, role and user changes, application decisions, results release, payment confirmation, adjustments, exports and document links (brief 13). Audit rows now carry the verified visitor address (D-195).

### 4.9 Dependencies

- CI fails on vulnerabilities our Go code reaches (`govulncheck`) and on high or critical advisories in shipped npm packages (D-193). Dependabot is on.
- **Fixed in this review:** `govulncheck` found GO-2026-5676 (quic-go HTTP/3 memory exhaustion, reachable through Gin) and the module was raised to v0.59.1. After the fix: 0 reachable. `npm audit`: 0 vulnerabilities, dev tools included (closes R-21).

## 5. Findings of this review

| ID | Finding | Severity | Status |
|---|---|---|---|
| F-01 | Any visitor could lock every account out of sign-in for 15 minutes at a time (shared per-address limit, R-23) | High (availability) | Fixed in code (D-195); active once Stage 19 sets the two variables |
| F-02 | No Content-Security-Policy, frame protection or HSTS on the site; few security headers on the API | Medium | Fixed (D-191, D-192) |
| F-03 | Permission routes accepted any account kind holding the permission; roles could be given to student or applicant accounts | Low (needs an admin mistake) | Fixed (D-196) |
| F-04 | Reachable vulnerability in quic-go (GO-2026-5676) | Medium | Fixed (v0.59.1) |
| F-05 | The virus scanner had never run locally (no arm64 image) and was untested against a real clamd | Medium (untested control) | Fixed for local and CI (D-194); production choice open |
| F-06 | Text typed into sign-in forms before the scripts loaded was wiped by hydration | Low (usability on slow phones) | Fixed (D-197) |

## 6. Open items before real data

These block go-live with real applicants. Each has an owner stage.

| Item | Stage or gate |
|---|---|
| The GitHub repository is public (R-10): make it private before any real data, client content or staging credentials | Before Stage 19 |
| Set `CLIENT_IP_HEADER` (the platform's client address header) and `CLIENT_IP_SECRET` on the frontend, and the same secret on the API | 19 |
| Decide production virus scanning (paid instance or scanner host), or accept and record "not scanned" with the client | 19, DG-04 |
| Retention schedule jobs, data subject request tools, breach procedure | 17c, DG-06 |
| Data protection registration, privacy notice approval, parental consent review (R-26) | DG-06, Stage 20 |
| Backups with a tested restore; uptime and error monitoring | 19 |
| Two-factor on GitHub, Cloudflare, Render, Supabase and the domain registrar (brief 13) | 19 (owner action) |
| HSTS `includeSubDomains` and preload once the domain is chosen | 19, DG-01 |
| KCB notification signature check | When KCB sends the key |
| One API instance, or a shared store for rate limits | 19 |

## 7. Not verified in this review

- Anything on real hosting (TLS settings, platform headers, the real client address header): there is no staging deploy yet.
- The upload path end to end through the API with a real ClamAV: the adapter was tested against a real clamd, and the API's handling of infected files is tested with a fake scanner.
- An external penetration test or automated web scanner: not run.
- Session fixation and cookie behaviour in browsers other than Chromium: Playwright here runs Chromium only.
