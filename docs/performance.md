# Performance

Measured in Stage 18 (2026-10-29). Budgets come from `CLAUDE.md` section 2: LCP under 2.5 s on a mid-range phone on 4G, and under about 150 KB of compressed JavaScript on a content page. AC-02 asks for Lighthouse mobile 90+ in all four categories on home, a programme page and admissions.

## How it was measured

- Lighthouse 12.6.0 (the same version as Stage 6b), mobile preset: simulated slow 4G (150 ms round trip, 1.6 Mbit/s) and a 4× slower CPU. Headless Chrome.
- Local production build (`next start`) against the local API and the development seed. No CDN, no Brotli: Next serves gzip.
- Each page is requested twice first, so the server is warm, then measured three times. The tables show the medians.
- These are lab numbers on a laptop, not field data from real phones. Repeat them on staging (Stage 19) with the real CDN.

A first run straight after the server started gave home 76 for performance, with 645 ms of blocking time. That came from the cold server: home measured on a warm server scores 94 to 95. The script now warms each page before measuring.

## Results

Performance / accessibility / best practices / SEO, after the Stage 18 changes:

| Page | Scores | LCP | FCP | TBT | CLS | JS (transferred) | Fonts |
|---|---|---|---|---|---|---|---|
| Home | 96/100/100/66 | 2.82 s | 0.91 s | 27 ms | 0 | 150 KB | 72 KB |
| Programme page | 96/100/100/69 | 2.74 s | 0.91 s | 52 ms | 0 | 157 KB | 72 KB |
| Admissions | 97/100/100/69 | 2.59 s | 0.76 s | 55 ms | 0 | 150 KB | 72 KB |
| Course finder | 97/100/100/69 | 2.67 s | 0.91 s | 60 ms | 0 | 150 KB | 72 KB |
| Apply | 97/100/100/69 | 2.67 s | 0.91 s | 64 ms | 0 | 150 KB | 72 KB |

Single runs on other pages: staff sign-in 97/100/100/66 (LCP 2.68 s, JS 165 KB), student sign-in 96/100/100/66 (2.67 s, 167 KB), application start 96/100/100/66 (2.83 s, 160 KB), contact 97/100/100/69 (2.67 s, 156 KB), FAQs 98/100/100/69 (2.11 s, 150 KB).

**SEO:** the 66 to 69 comes only from the pre-launch `noindex` (D-75). With `SITE_INDEXABLE=1` all five pages above score 100 for SEO.

Before the changes (single runs, warm server except home's first run): performance 95 to 97, LCP 2.74 to 3.04 s, fonts 105 KB.

## Against the budgets

| Budget | Status |
|---|---|
| Lighthouse mobile 90+ in all four categories (AC-02) | **Met** for performance, accessibility and best practices on every page measured; SEO 100 with indexing on, 66 to 69 until launch by design. |
| LCP under 2.5 s | **Not met** in this lab setup: 2.6 to 2.8 s on most pages (FAQs 2.11 s). See below. |
| About 150 KB compressed JS on a content page | **At the limit:** 150 to 157 KB gzip on content pages, 160 to 167 KB on sign-in and form pages. |
| No layout shift | **Met:** CLS 0 on every page. |

## What was changed

- **Headings font (D-218):** Newsreader was a 58 KB variable font, but headings use one weight, 600. It is now the static 600 file from fontsource (24 KB). The four places that used the serif at weight 400 (home motto, mission and vision; the "Staff portal" label) now ask for 600, which is what they would have rendered anyway. Fonts on every page drop from 105 KB to 72 KB; LCP improves by about 0.15 to 0.2 s.

## Where the remaining LCP time goes

The largest element paints 60 to 125 ms after the request on a laptop. The simulated 2.6 to 2.8 s is the time Lighthouse expects a slow phone on slow 4G to need to download and run what comes before that paint:

- Blocking all scripts in an experiment brought admissions from 2.74 s to 1.95 s. About 120 KB of the 150 KB is the React 19 and Next.js 16 runtime that every App Router page needs; the site's own code (menu, toasts, contact form, finder) is about 30 KB.
- Blocking the fonts brought it to 2.36 s, before the font change.
- Images are not the problem: every image audit passes (sizes, formats, off-screen loading), and the home hero has no load delay.

Bringing LCP under 2.5 s in this lab setup would mean removing most of the framework runtime from public pages, which is a change of architecture, not a fix. Recommended instead (R-22, R-25):

1. Measure on staging (Stage 19) with the CDN and Brotli (usually smaller than the gzip measured here; not measured yet), and record those numbers.
2. If staging is still above 2.5 s, consider rendering the public pages without client JavaScript except the menu and forms. Decide on that with the numbers from staging.

## Images and fonts

- Content images (programmes, schools, gallery, founders) are uploaded through the staff portal and served as resized variants with `srcset` (Stage 5b), lazy below the fold. Lighthouse finds nothing to improve.
- Brand files are SVG (11 to 51 KB, gzip 11 to 21 KB). The Open Graph image (62 KB PNG) is fetched only by link previews.
- Fonts: Inter variable, 48 KB, for body text; Newsreader 600, 24 KB, for headings; IBM Plex Mono 400 and 500 only on pages that show IDs, not preloaded.
- No third-party requests on any public page.
