# Handling requests about personal data

**Status: draft for the college to approve (DG-06).** The API for this is in place (Stage 17c); the **Data requests** screens come in Stage 17e. Written for the staff who handle these requests: the data protection contact, the Registrar and the Principal (permission `privacy.manage`). Where this page says [TO CONFIRM], the college's legal adviser should check it against the Data Protection Act, 2019 and its regulations.

People can ask the college to show them their data, correct it, delete it, stop using it, or give it to them in a form they can take elsewhere (privacy notice, "Your rights"). A parent or guardian may ask for a child.

## 1. Record the request the day it arrives

A request can arrive by email, letter, phone or in person, to anyone at the college. Whoever receives it passes it to the data protection contact the same day.

In the staff portal, open **Data requests** and record it:

- what kind of request it is (see the table below);
- who is asking and how to reach them;
- whether they ask for themselves or for someone else (for example "parent");
- the email address and phone number the college would hold for the person, so their records can be found;
- the date it arrived. The due date is filled in as 7 days later [TO CONFIRM the time limits]; change it if a different limit applies.

Each request gets a reference such as DTC-DSR-2026-0001. Give it to the person.

| Kind | What the person asks | What to do |
|---|---|---|
| Access | To see the data held about them | Export the data (step 3) |
| Portability | Their data in a form they can take elsewhere | Export the data (step 3) |
| Correction | To fix wrong or incomplete data | Correct it on the record (step 4) |
| Erasure | To delete their data | Erase what can be erased (step 5) |
| Objection | To stop a use of their data | Decide with the Principal; record the decision |
| Withdraw consent | To withdraw consent they gave, for example to a photo | Remove the photo or stop the use; record what was done |
| Other | Anything else about their data | Record what was asked and what was done |

## 2. Check who is asking

Before sending any data or erasing anything, make sure the person asking is the person (or their parent or guardian):

- in person: see their national ID, passport or student card;
- by phone or email: contact them back on the phone number or email address the college already holds, not one given in the request;
- for a child: also check the parent or guardian is the one named on the record.

Record how you checked ("Saw national ID at the front office"). The portal refuses exports and erasure until you do.

If you cannot confirm who is asking, refuse the request and say why.

## 3. Access and portability: export the data

**Export data** downloads one JSON file with everything the system holds that carries the request's email address or phone number: applications (with answers, documents listed by name, history, offers and staff notes), student records (registration, released results, invoices and payments), enquiries, records naming the person as a parent or guardian, staff accounts and messages sent.

Before sending it:

1. Check each record really belongs to the person. Two people can share a phone number.
2. Remove anything about other people, such as another person's name in a staff note.
3. Send it securely: hand it over in person, or email it as a password-protected file and give the password by phone [TO CONFIRM the college's preferred method].

Uploaded documents are listed but not included. If the person wants copies, download them from the application in the admissions screens.

Data kept outside the system (paper files, spreadsheets, email) is not in the export. Ask the offices that may hold it and add what they find.

## 4. Correction

Correct the data where it lives:

- an applicant's details: they can edit their own application until they submit it; after that, note the correction on the application;
- a student's details: through the student record screens;
- anything else: ask the developer or the ICT Admin.

Record what was corrected as a note on the request.

## 5. Erasure

The person's records are listed on the request. You can erase:

- **an applicant who did not enrol**, once no application is still being decided. This removes their personal details, documents and answers but keeps the application reference and outcome for counts (see `docs/retention.md`, "What erasing an applicant means");
- **an enquiry**, which deletes it with its staff notes.

You cannot erase:

- **a student's records**, including their applicant record. The college keeps student, academic and financial records [TO CONFIRM the legal basis]. Refuse that part of the request and give the reason;
- **an applicant with an application still being decided.** Ask whether they want to withdraw it. Once it is withdrawn or decided, erase.

Erasure cannot be undone.

## 6. Close the request

When it is done, close the request as **completed** with what was done, or **refused** with the reason. The outcome is what you tell the person. Reply before the due date. If you need more time, tell the person before the due date and why [TO CONFIRM whether and how long an extension is allowed].

Tell the person they can complain to the Office of the Data Protection Commissioner if they are not happy with the answer.

## Records

Every step is recorded on the request with who did it and when, and in the audit log. The register is evidence that requests were answered; keep it.
