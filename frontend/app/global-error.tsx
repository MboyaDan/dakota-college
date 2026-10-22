"use client";

import "./globals.css";

/** Last resort when the root layout itself fails. Plain, with no data needed. */
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <html lang="en">
      <body className="flex min-h-screen items-center justify-center bg-ivory p-6 text-ink">
        <main className="flex max-w-xl flex-col gap-4">
          <h1 className="text-3xl font-semibold text-navy-900">Dakota Training College</h1>
          <p className="text-lg">
            Something went wrong on our side and the page could not be shown. Please try again.
          </p>
          {error.digest ? (
            <p className="text-sm text-ink-muted">
              Reference: <span className="font-mono">{error.digest}</span>
            </p>
          ) : null}
          <button
            type="button"
            onClick={() => reset()}
            className="min-h-touch self-start rounded-control bg-gold px-5 py-2 font-semibold text-navy-900"
          >
            Try again
          </button>
        </main>
      </body>
    </html>
  );
}
