# Data retention schedule

**Status: draft for the college to approve (DG-06).** The periods below are defaults (DQ-18, A-48). Nothing personal is erased in production until the college approves them and the API is set to `RETENTION_MODE=apply`.

This is the schedule the API follows. The code that applies it is `backend/internal/privacy/retention.go`; if the two ever differ, the code is what runs, and this page must be corrected.

## What is removed, and when

| Records | Kept for | What happens after |
|---|---|---|
| Spent sign-in, password reset, two-step and download links | 1 day after they expire | Deleted |
| Ended sign-in sessions (they hold the internet address and browser) | 30 days after they end | Deleted |
| Records that stop a form being sent twice | Until they expire | Deleted |
| Unfinished applications | Until the draft expires (60 days after the last save, A-18) | Marked expired, with a history row |
| Log of retention runs | 24 months | Deleted |
| Record of emails and text messages sent (recipient, which message, when) | 12 months after sending | Deleted. The message text itself is deleted as soon as the message is sent or given up (Stage 16a). |
| Enquiries from the contact form and other channels, with staff notes | 12 months after the enquiry is closed | Deleted. Open enquiries are kept until someone closes them. |
| Applicants who did not enrol | 12 months after their last application ended (rejected, withdrawn or expired), or after its intake started, whichever is later | Erased (see below) |

The first five rows are housekeeping: those records have no use once spent, so they are removed even in report mode.

## What is kept

| Records | Kept for | Why |
|---|---|---|
| Student records: personal details, guardians, registration, status history | Permanently [TO CONFIRM] | To confirm qualifications in future (privacy notice draft). |
| Results and transcripts | Permanently [TO CONFIRM] | Same. |
| Invoices, payments, receipts, adjustments, M-Pesa and KCB records | As long as tax and accounting law requires [TO CONFIRM the period with the college's accountant] | Legal obligation. Not erased by this schedule. |
| Audit log | Not erased by this schedule [TO CONFIRM a period] | Accountability: who did what, when. It holds internet addresses and record IDs, never names, emails or message text. |
| Staff accounts | While the person works for the college; then disabled, not deleted | Their actions in the audit log must stay attributable. |
| Consent records | With the record they belong to; names and addresses blanked when an applicant is erased | Evidence of what was agreed. |
| Register of data protection requests | Not erased by this schedule [TO CONFIRM a period] | Evidence that requests were answered. |

## What erasing an applicant means

Erasing keeps the applicant and application rows so references and counts still work, and removes everything that identifies the person:

- name, email, phone, date of birth, identity number, nationality and county are blanked or replaced with "Erased";
- guardians, education history and uploaded documents are deleted, and the files are removed from storage;
- answers saved at submission, sponsor details, offer conditions and offer letters are removed;
- staff notes, information requests and the reasons in the status history are replaced with "[Erased]";
- consent records keep what was agreed and when, but the name, internet address and browser are blanked;
- sign-in links, sessions and messages to the person are deleted, and their sign-in account is disabled.

What stays: the application reference, programme, intake, status, dates and who on the staff handled it. The audit log records that the erasure happened, with no personal details.

An applicant who became a student is never erased this way: their applicant record is part of the student record. An applicant with an application still being decided is not erased until it is decided or withdrawn.

## How it runs

- The API runs the schedule once a day, starting two minutes after it starts. Several copies of the API can run; a database lock lets one run at a time.
- `RETENTION_MODE` (API environment) decides what a run does:
  - `report` (the default): housekeeping is removed; everything else is counted, not erased.
  - `apply`: everything due is removed or erased.
  - `off`: nothing runs.
- Each run is logged (`retention_runs`) with what each rule found. An applied run is also audited (`privacy.retention_applied`), and each erased applicant separately (`privacy.applicant_erased`).
- At most 200 applicants are erased per run, so a first run over years of data stays short. The rest are erased on the following days.
- Staff with `privacy.manage` can see the schedule and the run log, and count what is due now. They cannot apply the schedule from the portal.
- From a developer machine, with the target environment's `DATABASE_URL` and S3 settings (as for migrations): `go run ./cmd/retention` prints a report; `go run ./cmd/retention apply` applies the schedule once.

## Before turning on apply

1. The college approves the periods on this page (DG-06), or gives different ones; a developer changes `Schedule` in `retention.go` and this page together.
2. Run `go run ./cmd/retention` with production's settings and check the counts make sense.
3. Set `RETENTION_MODE=apply` on the API and restart it.
4. Update the "How long we keep it" section of the privacy notice to match.
