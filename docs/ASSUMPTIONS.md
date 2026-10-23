# Assumptions

Every default the build runs on. Nothing here is a confirmed fact about the college until its status says **Confirmed**, with who confirmed it and when. Anything about accreditation, results, fees, statistics or legal status must come from the client.

Status values: **Open** (default in use, not confirmed), **Confirmed**, **Replaced** (superseded; see note).

## From the brief (section 19)

| ID | Assumption | Status |
|---|---|---|
| A-01 | One campus, one currency (KES), Africa/Nairobi time zone. | Open |
| A-02 | The college can supply content, fees and photos before M1 launch. | Open |
| A-03 | Applicants mostly use phones. | Open |
| A-04 | Fees are set per programme and intake and are published. | Open |
| A-05 | The college will pay for production-grade hosting before real student data is stored. | Open |
| A-06 | Student numbers and receipt numbers are gapless and unique. | Open |
| A-07 | Free tiers are used for development and private staging only. | Open |

## Discovery question defaults (brief section 17)

| ID | Question (short) | Default in use | Status |
|---|---|---|---|
| DQ-01 | Official name, registration, address, phone, email, socials, office hours | "Dakota Training College", Makueni. Owner asked for editable dummy values: the **dev seed only** fills them with obviously fake values marked "(Sample)" (`.invalid` emails, test phone numbers). Migrations leave them empty, so production shows nothing until staff enter real values. **Phone and WhatsApp: +254 729 041996** (owner, 2026-10-22), to be entered in Site settings at go-live (`docs/launch-content.md`). Address, emails, office hours and socials still to come. | Partly answered (owner, 2026-10-22) |
| DQ-02 | Meaning of "School of Pool Management" | Use the name as given. The school's description and programmes are left empty and editable so the college can add what it teaches in the staff portal. No sample programmes are invented for it. | Confirmed (owner, 2026-10-22) |
| DQ-03 | Programme list per school | Sample programmes in development seed only, marked "(Sample)", removed before launch. Real programmes are entered by staff in the content manager because they change. | Confirmed (owner, 2026-10-22) |
| DQ-04 | TVETA, CDACC, KNEC numbers | Owner asked for editable dummy values. The dev seed creates one record labelled "SAMPLE, not a real registration number". Publishing an accreditation item still needs a verification date (NN-01). Hidden in production until real, verified values are entered. | Confirmed (owner, 2026-10-22) |
| DQ-05 | Intake months and deadlines | Intakes are admin data; none shown until entered. Dev seed has sample intakes. | Confirmed (owner, 2026-10-22) |
| DQ-06 | Application fee | No fee. Optional later (Could). | Open |
| DQ-07 | Assessment model | Configurable per programme; marks and grade bands by default. | Open |
| DQ-08 | Student number format | `DTC/YYYY/NNNN`, gapless per year, editable setting. | Open |
| DQ-09 | How fees are collected today | Cash, bank and manual M-Pesa recorded as pending confirmation. | Open |
| DQ-10 | Accommodation, transport, meals | Not modelled in v1. | Open |
| DQ-11 | Leadership profiles and consent | Section hidden until supplied. | Open |
| DQ-12 | Real photos with consent | Neutral placeholders, clearly marked. | Open |
| DQ-13 | Languages | **English only for now** (owner, 2026-10-22, replaces the earlier approval). No Kiswahili toggle is built in Stage 7. Content is not structured in a way that blocks adding Kiswahili later. | Confirmed (owner, 2026-10-22) |
| DQ-14 | Who edits and who approves content | Content Editor writes; Registrar or Principal approves (publish permission). | Confirmed (owner, 2026-10-22) |
| DQ-15 | Existing student data to import | None in v1. CSV import is a Could. | Open |
| DQ-16 | Staff list, Super Admin, recovery contact | Starter roles from brief section 11; developer holds Super Admin until handover. | Confirmed (owner, 2026-10-22) |
| DQ-17 | Budget | Planning figure from brief 12.2 (about USD 45 to 60 per month); recheck before quoting. | Open |
| DQ-18 | Retention periods, data protection contact | Applications kept 12 months if not enrolled; student records permanent. | Open |
| DQ-19 | Go-live date and approvers | None assumed. | Open |

## New in Stage 0

| ID | Assumption | Status |
|---|---|---|
| A-08 | The logo files in `docs/brand/dakota-logo/` are the approved vector rebuild described in brief 14.1. They are used as supplied and never redrawn. | Confirmed (owner, 2026-10-22) |
| A-09 | The tagline is "Turning Potential into Progress" and the ribbon text "SKILLS · DISCIPLINE · OPPORTUNITY" is correct. The tagline artwork reads "Progression", so `docs/brand/retagline.py` derives "Progress" versions from the original letter outlines (D-86, R-24). | Confirmed (owner, 2026-10-22) |
| A-10 | Kiswahili deferred (see DQ-13). No language columns in Stage 3; adding Kiswahili later means a translations table, not a schema rewrite. | Replaced by DQ-13 answer (2026-10-22) |
| A-11 | The first Super Admin email for local development is a `.invalid` address set in `.env`; the real one comes from the client at handover (DQ-16). | Confirmed (owner, 2026-10-22) |
| A-12 | The GitHub repository stays public during scaffold stages and becomes private before Stage 5. Making it private does not change commits or the workflow; the only effect is that GitHub Actions on a free private repo is capped at 2,000 minutes a month, which this CI fits within. | Confirmed (owner, 2026-10-22) |
| A-13 | Toolchain: Go 1.26 (D-33; the local 1.25 install downloads 1.26.10 automatically via `toolchain`), Node 24, npm, Docker 29, PostgreSQL client. | Open |
| A-14 | Staging gets no seed data: the seed refuses anything but development and test (CLAUDE.md). A staging demo will need a separate, clearly fake demo-data decision in Stage 19. | Open |
| A-15 | Optional TOTP for finance, registrar and admin accounts is built in Stage 17 (brief 13), not Stage 4. | Open |
| A-16 | Applicant sign-in links work for 30 minutes, and an address can ask for 5 an hour (D-97). | Confirmed (owner, 2026-10-23) |
| A-17 | Registrar and ICT Admin (and Super Admin) may reword messages to applicants (`notifications.manage`); Admissions Officers may not. | Confirmed (owner, 2026-10-23); the owner will review the starter wording later |
| A-18 | An unsubmitted draft expires 60 days after its last save while `applications.draft_expiry_days` is empty; staff can set 1 to 365 days in Site settings. | Confirmed (owner, 2026-10-24) |
| A-19 | Parental consent for under-18 applicants is captured as the applicant's declaration that the named parent or guardian has read the privacy notice and agrees, recorded in the guardian's name (D-106). The guardian does not confirm it themselves. Whether this is enough under the Data Protection Act is for the DG-06 review. | Accepted by owner (2026-10-24); to be raised in the DG-06 review (R-26) |
| A-20 | Required before submission: names, date of birth, identity document type and number, phone, nationality, county; for under-18s the guardian's name, relationship, phone and agreement; at least one school with a KCSE mean grade if KCSE; who pays the fees (name and phone unless self). Middle name, guardian email, subject grades and year finished are optional. | Confirmed (owner, 2026-10-24) |
| A-21 | Documents required before submission: the identity document chosen in About you (national ID, birth certificate or passport), a passport-size photo, and the KCSE certificate or result slip when KCSE is listed. Other certificates are optional. PDF, JPG or PNG, up to 10 MB each, at most 12 per application. | Default, Stage 9b; owner may change |
