import type { ReactNode } from "react";
import { ChevronDown } from "@/components/icons";
import { cn } from "@/lib/cn";

// Built on <details>/<summary>: keyboard and screen-reader support with no
// JavaScript, and content stays findable with the browser's find-in-page.

export function Accordion({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn("divide-y divide-stone rounded-card border border-stone bg-white", className)}>
      {children}
    </div>
  );
}

type AccordionItemProps = {
  title: ReactNode;
  children: ReactNode;
  defaultOpen?: boolean;
  /** Items sharing a name behave exclusively: opening one closes the others. */
  name?: string;
};

export function AccordionItem({ title, children, defaultOpen, name }: AccordionItemProps) {
  return (
    <details className="group" open={defaultOpen} name={name}>
      <summary className="flex min-h-touch cursor-pointer list-none items-center justify-between gap-4 px-4 py-3 font-semibold text-navy-900 [&::-webkit-details-marker]:hidden">
        <span>{title}</span>
        <ChevronDown aria-hidden className="size-5 shrink-0 transition-transform group-open:rotate-180" />
      </summary>
      <div className="px-4 pb-4 text-ink">{children}</div>
    </details>
  );
}
