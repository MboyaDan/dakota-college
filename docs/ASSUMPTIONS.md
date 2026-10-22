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
| DQ-01 | Official name, registration, address, phone, email, socials, office hours | "Dakota Training College", Makueni. All contact fields empty and hidden until supplied. | Open |
| DQ-02 | Meaning of "School of Pool Management" | Use the name as given; no programme content for it until explained. | Open |
| DQ-03 | Programme list per school | Sample programmes in development seed only, marked "(Sample)", removed before launch. | Open |
| DQ-04 | TVETA, CDACC, KNEC numbers | Accreditation section hidden until verified. | Open |
| DQ-05 | Intake months and deadlines | Intakes are admin data; none shown until entered. | Open |
| DQ-06 | Application fee | No fee. Optional later (Could). | Open |
| DQ-07 | Assessment model | Configurable per programme; marks and grade bands by default. | Open |
| DQ-08 | Student number format | `DTC/YYYY/NNNN`, gapless per year, editable setting. | Open |
| DQ-09 | How fees are collected today | Cash, bank and manual M-Pesa recorded as pending confirmation. | Open |
| DQ-10 | Accommodation, transport, meals | Not modelled in v1. | Open |
| DQ-11 | Leadership profiles and consent | Section hidden until supplied. | Open |
| DQ-12 | Real photos with consent | Neutral placeholders, clearly marked. | Open |
| DQ-13 | Languages | English. Kiswahili for key admissions pages is a Should, built only if approved (see A-10). | Open |
| DQ-14 | Who edits and who approves content | Content Editor writes; Registrar or Principal approves (publish permission). | Open |
| DQ-15 | Existing student data to import | None in v1. CSV import is a Could. | Open |
| DQ-16 | Staff list, Super Admin, recovery contact | Starter roles from brief section 11; developer holds Super Admin until handover. | Open |
| DQ-17 | Budget | Planning figure from brief 12.2 (about USD 45 to 60 per month); recheck before quoting. | Open |
| DQ-18 | Retention periods, data protection contact | Applications kept 12 months if not enrolled; student records permanent. | Open |
| DQ-19 | Go-live date and approvers | None assumed. | Open |

## New in Stage 0

| ID | Assumption | Status |
|---|---|---|
| A-08 | The logo files in `docs/brand/dakota-logo/` are the approved vector rebuild described in brief 14.1. They are used as supplied and never redrawn. | Open |
| A-09 | The tagline "Turning Potential into Progression" (on `dakota-logo-*-tagline.svg`) and the ribbon text "SKILLS · DISCIPLINE · OPPORTUNITY" are correct. Brief 14.1 says the last tagline word was unreadable on the original. Until confirmed, the site uses the logo variants **without** the tagline and does not show the tagline as text. | Open |
| A-10 | Kiswahili toggle (CLAUDE.md Stage 7) is **not approved** yet. Content tables get a language-ready shape where cheap, but no Kiswahili text is produced, and none is machine-translated. | Open |
| A-11 | The first Super Admin email for local development is a `.invalid` address set in `.env`; the real one comes from the client at handover (DQ-16). | Open |
| A-12 | The GitHub repository stays public during scaffold stages and becomes private before any client content, real data or staging credentials are added (R-10). | Open |
| A-13 | Toolchain on the developer machine: Go 1.25, Node 24, npm, Docker 29, PostgreSQL client. Versions are pinned in Stage 1. | Open |
