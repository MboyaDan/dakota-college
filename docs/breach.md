# Personal data breach procedure

**Status: draft for the college to approve (DG-06).** Written for the Principal, the data protection contact, the ICT Admin and the developer on support. Where this page says [TO CONFIRM], the college's legal adviser should check it against the Data Protection Act, 2019 and its regulations.

A personal data breach is anything that exposes, loses, changes or destroys personal data without permission. Examples:

- someone signs in with a staff member's password;
- an export or document is emailed to the wrong person;
- a laptop or phone with a staff session or downloaded files is lost or stolen;
- a bug shows one applicant or student another person's data;
- the database, file storage or a backup is reached by someone who should not reach it;
- data is deleted or changed by mistake and cannot be restored.

If in doubt, treat it as a breach and follow this page. Reporting something that turns out to be harmless costs little; not reporting a real breach can cost a lot.

## The clock

Under section 43 of the Act, the college must tell the Data Protection Commissioner within **72 hours** of becoming aware of a breach that is likely to put people at real risk of harm, and must tell the people affected, in writing, within a reasonable time. A service provider that processes data for the college must tell the college within 48 hours [TO CONFIRM both periods and when the 72 hours start].

The 72 hours run from when the college becomes aware, including weekends. Write down the date and time you first heard about it.

## 1. Report it at once (anyone)

Whoever notices a breach tells the Principal and the data protection contact straight away, by phone, then in writing. Say what happened, when, what data and how many people may be affected, and what you have already done. Do not try to investigate alone, and do not delete anything: logs and emails are evidence.

[TO CONFIRM: names and phone numbers of the Principal, the data protection contact, the ICT Admin and the developer on support.]

## 2. Contain it (ICT Admin and developer, first hours)

Stop it getting worse. What fits the case:

| What happened | What to do in this system |
|---|---|
| A staff account was used by someone else | In **Staff schools**, disable the account; that ends its sessions. Reset its two-step sign-in if it had it. Ask the person to choose a new password once the account is back on. |
| Many accounts may be affected | Disable the affected accounts. If needed, end every session by running `DELETE FROM sessions;` on the database (everyone signs in again). |
| A file or export went to the wrong person | Ask them in writing to delete it and confirm. Download links expire in minutes; check the audit log for the link (`file.link_issued`, `privacy.request_exported`). |
| A secret leaked (database password, storage keys, M-Pesa or KCB secrets, `MESSAGE_KEY`, `TOTP_KEY`, `CLIENT_IP_SECRET`) | Rotate it at the provider and in the API environment, then restart the API. Changing `TOTP_KEY` turns off every authenticator (R-29); changing `MESSAGE_KEY` loses queued messages. Check `docs/security.md` for each secret. |
| A bug exposes data | Turn off the affected feature or take the site down while the developer fixes it. The public pages still render from the fallback snapshot when the API is down. |
| Data was deleted or changed | Stop further changes; restore from backup into a separate database first and compare (runbook, Stage 19). |

Record every action with the time and who did it.

## 3. Assess it (Principal and data protection contact, within 24 hours)

Work out, as far as you can:

- what personal data was involved, and whose (applicants, students, children, guardians, staff);
- how many people;
- whether the data was actually seen or taken, or only exposed;
- what harm could follow: identity fraud, financial loss, distress, discrimination, physical risk. Data about children, identity numbers, health information and financial details raise the risk;
- whether it is still going on.

Useful evidence in the system:

- the audit log: who signed in, from which address, and what they did (`auth.login_succeeded`, `auth.login_failed`, exports, document opens, changes);
- the API and hosting provider logs;
- the retention run log, if data went missing.

## 4. Notify (Principal, within 72 hours of becoming aware)

**The Data Protection Commissioner**, if the breach is likely to put people at real risk of harm [TO CONFIRM the test and the reporting channel; see www.odpc.go.ke]. Report what you know even if the investigation is not finished, and add details later. Include:

- what happened and when, and when you found out;
- the data and the people affected, and roughly how many;
- the likely consequences;
- what you have done and will do to deal with it and to reduce harm;
- who to contact at the college.

**The people affected**, in writing, in plain language, as soon as you reasonably can: what happened, what data, what it could mean for them, what the college has done, what they can do (for example watch for messages pretending to be the college, change a password), and who to contact. For a child, write to the parent or guardian.

If you decide not to notify, write down why.

## 5. Learn from it (within two weeks)

- Find the cause and fix it.
- Check whether other data or systems have the same weakness.
- Update `docs/security.md` and this procedure if needed.
- Brief the staff involved.

## Breach log

Keep a record of every breach, notified or not [TO CONFIRM where: a restricted document is enough to start]. For each:

| Field | |
|---|---|
| Date and time found, and by whom | |
| What happened | |
| Data and people affected | |
| Containment actions, with times | |
| Risk assessment | |
| Commissioner notified? When, reference, or why not | |
| People notified? When, how, or why not | |
| Cause and fix | |
| Closed on | |
