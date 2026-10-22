"use client";

import Link from "next/link";
import { useEffect } from "react";
import { Button } from "@/components/ui/button";

/** Something failed while showing a public page (HTTP 500). */
export default function SiteError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-16 lg:px-6">
      <p className="font-semibold text-royal">Something went wrong</p>
      <h1 className="text-4xl font-semibold text-navy-900">This page could not be shown</h1>
      <p className="text-lg">
        It is a problem on our side, not something you did. Try again in a moment. If it keeps happening,
        contact us and tell us which page you were on.
      </p>
      {error.digest ? (
        <p className="text-sm text-ink-muted">
          Reference: <span className="font-mono">{error.digest}</span>
        </p>
      ) : null}
      <div className="flex flex-wrap items-center gap-4">
        <Button onClick={() => reset()}>Try again</Button>
        <Link href="/" className="inline-flex min-h-touch items-center font-semibold text-royal underline">
          Go to the home page
        </Link>
        <Link
          href="/contact"
          className="inline-flex min-h-touch items-center font-semibold text-royal underline"
        >
          Contact us
        </Link>
      </div>
    </div>
  );
}
