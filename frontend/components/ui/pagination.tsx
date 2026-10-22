import Link from "next/link";
import { ChevronLeft, ChevronRight } from "@/components/icons";
import { cn } from "@/lib/cn";
import { pageSlots } from "@/lib/pagination";

type PaginationProps = {
  page: number;
  totalPages: number;
  /** Builds the URL for a page, e.g. (p) => `/news?page=${p}`. */
  hrefFor: (page: number) => string;
  label?: string;
};

const cell =
  "inline-flex min-h-touch min-w-touch items-center justify-center rounded-control px-3 font-semibold";

export function Pagination({ page, totalPages, hrefFor, label = "Pages" }: PaginationProps) {
  if (totalPages <= 1) return null;
  const slots = pageSlots(page, totalPages);

  return (
    <nav aria-label={label}>
      <ul className="flex flex-wrap items-center gap-1">
        <li>
          {page > 1 ? (
            <Link href={hrefFor(page - 1)} className={cn(cell, "gap-1 text-royal hover:bg-royal/5")}>
              <ChevronLeft aria-hidden className="size-5" />
              Previous
            </Link>
          ) : null}
        </li>
        {slots.map((slot, i) =>
          slot === "gap" ? (
            <li key={`gap-${i}`} aria-hidden className={cn(cell, "text-ink-muted")}>
              …
            </li>
          ) : (
            <li key={slot}>
              <Link
                href={hrefFor(slot)}
                aria-current={slot === page ? "page" : undefined}
                aria-label={`Page ${slot}`}
                className={cn(cell, slot === page ? "bg-navy-900 text-white" : "text-royal hover:bg-royal/5")}
              >
                {slot}
              </Link>
            </li>
          ),
        )}
        <li>
          {page < totalPages ? (
            <Link href={hrefFor(page + 1)} className={cn(cell, "gap-1 text-royal hover:bg-royal/5")}>
              Next
              <ChevronRight aria-hidden className="size-5" />
            </Link>
          ) : null}
        </li>
      </ul>
    </nav>
  );
}
