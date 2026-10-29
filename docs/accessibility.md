# Accessibility

Status at Stage 18 (2026-10-29). Target: WCAG 2.1 level AA (brief section 2 UX principles, AC-02 accessibility score). The public accessibility statement is a draft in `docs/legal/accessibility-statement.md`.

## Automated checks (run on every pull request in CI)

| Check | Where | Result |
|---|---|---|
| axe (WCAG 2.1 A and AA rules; fails on serious or critical) at 360 px and desktop | Every public page (27 paths, including the 404 page, in `public.spec.ts`), the design page, staff sign-in and two-step screens, applicant sign-in and every step of the application form, the student portal, and the staff screens for admissions, academics, results, finance, M-Pesa, KCB, enquiries, reports and data requests | No serious or critical issues |
| No sideways scrolling at 360 px | The same pages | None |
| Lighthouse accessibility, mobile | Home, programme page, admissions, finder, apply, contact, FAQs, staff and student sign-in, application start | 100 on every page measured |
| Keyboard (new, `keyboard.spec.ts`) | Home, finder, admissions, fees, FAQs, contact, apply, about | See below |

Automated tools find only part of the problems. The manual checks below are still needed.

## Keyboard (automated in Stage 18)

`keyboard.spec.ts`, at 360 px and desktop:

- The first Tab lands on "Skip to main content", which is visible when focused and moves focus to the main content.
- Tabbing through home, the finder, contact and FAQs (up to 80 stops each): every stop is on screen and shows a focus ring (3 px outline).
- The main menu (desktop) and the mobile menu open with Enter, their links are reachable with Tab, and Escape closes them and returns focus to the button.
- An FAQ opens and closes with Enter and Space (desktop; see the note below).
- The contact form can be sent with the keyboard; with empty fields, focus moves to the first field to fix, which is marked invalid and says what to do.

Earlier stages already test the design-page components with the keyboard (dialog Escape, tabs with arrow keys, the skip link). Focus after an error on the application form and the staff sign-in screens is not tested automatically (manual checklist, S4).

**Note:** in Chromium's phone emulation the FAQ questions (`<summary>` elements) are not in the tab order, even with `tabindex`. On desktop they are. Phone screen readers reach them as buttons through the accessibility tree, not through Tab. Whether a real Android phone with a keyboard can Tab to them is not verified (manual checklist, item K6).

## Manual checklist (not yet done)

These need a person with the device or screen reader. Not run in Stage 18: there is no screen reader in this environment, and results from an emulator would not be evidence. Record each result with the date, device, browser and screen reader version.

### Keyboard

| # | Check | Pages |
|---|---|---|
| K1 | Every action can be done without a mouse, in a sensible order | Apply from home to submission; staff: review an application, record a payment, release results |
| K2 | Focus never gets stuck, and dialogs return focus where they opened | Staff dialogs (two-step reset, erase, offers) |
| K3 | Focus ring visible on navy backgrounds (header, footer, finder band) | Home |
| K4 | Date fields and selects work with the keyboard in Chrome, Firefox and Safari | Application form, data request form |
| K5 | Nothing moves or changes when a field gets focus | All forms |
| K6 | FAQ questions reachable with Tab on a real Android phone with a Bluetooth keyboard | FAQs |

### Screen readers

Test with NVDA and Firefox on Windows, VoiceOver and Safari on iPhone, and TalkBack and Chrome on Android (the most likely device for applicants).

| # | Check | Pages |
|---|---|---|
| S1 | Page title and one h1 are read first; headings give the page outline | Every public page |
| S2 | Landmarks: banner, main navigation, main, footer; the skip link works | Home |
| S3 | Menus announce expanded and collapsed | Header, mobile menu |
| S4 | Form labels, hints and errors are read with each field; the error summary or first error is announced after sending | Contact, application steps, staff sign-in |
| S5 | Status messages (toasts, "Saved", "Downloaded") are announced without moving focus | Application autosave, staff screens |
| S6 | Images: content photos read their description; decorative artwork is skipped | Home, schools, gallery |
| S7 | Tables read their caption and column headers | Fees, staff reports, finance statements |
| S8 | Status tags (success, danger) are read with their words, not only their colour | Staff lists, student portal |
| S9 | The two-step code field and recovery code option are clear | Staff sign-in |

### Other

| # | Check |
|---|---|
| O1 | Zoom to 200 % and 400 % on desktop: no content lost, no sideways scrolling at 400 % (reflow) |
| O2 | Phone in landscape: nothing cut off |
| O3 | Windows high contrast mode: buttons, fields and focus remain visible |
| O4 | Reduced motion setting: no animation (CSS honours `prefers-reduced-motion`) |
| O5 | Plain language: a person not from the college reads the admissions and fees pages and can say what they need to apply |

## Known limits

- Text in uploaded documents (PDF offer letters, transcripts) has not been checked with a screen reader.
- Content written by staff (news, programme descriptions) is only as accessible as it is written: the content editor asks for image descriptions and headings, but cannot check the wording.
