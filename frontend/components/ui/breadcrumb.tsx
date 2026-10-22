import Link from "next/link";
import { ChevronRight } from "@/components/icons";

export type Crumb = { label: string; href?: string };

/** The last crumb is the current page and is not a link. */
export function Breadcrumb({ items }: { items: Crumb[] }) {
  return (
    <nav aria-label="Breadcrumb">
      <ol className="flex flex-wrap items-center gap-x-1 gap-y-1 text-sm">
        {items.map((item, i) => {
          const last = i === items.length - 1;
          return (
            <li key={`${item.label}-${i}`} className="flex items-center gap-1">
              {item.href && !last ? (
                <Link
                  href={item.href}
                  className="inline-flex min-h-touch items-center text-royal underline underline-offset-2 hover:no-underline"
                >
                  {item.label}
                </Link>
              ) : (
                <span aria-current={last ? "page" : undefined} className="text-ink-muted">
                  {item.label}
                </span>
              )}
              {last ? null : <ChevronRight aria-hidden className="size-4 text-ink-muted" />}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
