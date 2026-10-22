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
| DQ-01 | Official name, registration, address, phone, email, socials, office hours | "Dakota Training College", Makueni. Owner asked for editable dummy values: the **dev seed only** fills them with obviously fake values marked "(Sample)" (`.invalid` emails, test phone numbers). Migrations leave them empty, so production shows nothing until staff enter real values. | Confirmed (owner, 2026-10-22) |
| DQ-02 | Meaning of "School of Pool Management" | Use the name as given; no programme content for it until explained. | Open |
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
| DQ-13 | Languages | English plus a Kiswahili toggle on key admissions pages, **approved**. Kiswahili text comes from a human translator; until a page has a translation, the toggle shows the English text with a note. No machine translation. | Confirmed (owner, 2026-10-22) |
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
| A-09 | The tagline "Turning Potential into Progression" and the ribbon text "SKILLS · DISCIPLINE · OPPORTUNITY" are correct, so tagline logo variants may be used. | Confirmed (owner, 2026-10-22) |
| A-10 | Kiswahili toggle approved (see DQ-13). Content tables for key admissions pages get a language column from Stage 3. | Replaced by DQ-13 answer (2026-10-22) |
| A-11 | The first Super Admin email for local development is a `.invalid` address set in `.env`; the real one comes from the client at handover (DQ-16). | Confirmed (owner, 2026-10-22) |
| A-12 | The GitHub repository stays public during scaffold stages and becomes private before Stage 5. Making it private does not change commits or the workflow; the only effect is that GitHub Actions on a free private repo is capped at 2,000 minutes a month, which this CI fits within. | Confirmed (owner, 2026-10-22) |
| A-13 | Toolchain on the developer machine: Go 1.25, Node 24, npm, Docker 29, PostgreSQL client. Versions are pinned in Stage 1. | Open |
