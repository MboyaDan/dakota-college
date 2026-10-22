import type { ReactNode } from "react";
import { Breadcrumb, type Crumb } from "@/components/ui/breadcrumb";

/** Breadcrumb, the page's one h1, and an optional lead paragraph. */
export function PageHeader({
  crumbs,
  title,
  lead,
  children,
}: {
  crumbs: Crumb[];
  title: string;
  lead?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className="border-b border-stone bg-white">
      <div className="mx-auto flex max-w-6xl flex-col gap-3 px-4 py-6 lg:px-6 lg:py-10">
        <Breadcrumb items={crumbs} />
        <h1 className="text-3xl font-semibold text-navy-900 sm:text-4xl">{title}</h1>
        {lead ? <div className="max-w-prose text-lg text-ink">{lead}</div> : null}
        {children}
      </div>
    </div>
  );
}

/** A page section with a heading; renders nothing when it has no content. */
export function Section({
  id,
  title,
  children,
  className,
}: {
  id?: string;
  title: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section id={id} aria-labelledby={id ? `${id}-title` : undefined} className={className}>
      <div className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-10 lg:px-6">
        <h2 id={id ? `${id}-title` : undefined} className="text-2xl font-semibold text-navy-900 sm:text-3xl">
          {title}
        </h2>
        {children}
      </div>
    </section>
  );
}

/** Plain text from the content manager, with blank lines as paragraphs. */
export function Prose({ text, className }: { text: string; className?: string }) {
  const paragraphs = text
    .split(/\n\s*\n/)
    .map((p) => p.trim())
    .filter(Boolean);
  if (paragraphs.length === 0) return null;
  return (
    <div className={className ?? "flex max-w-prose flex-col gap-3"}>
      {paragraphs.map((p, i) => (
        <p key={i} className="whitespace-pre-line">
          {p}
        </p>
      ))}
    </div>
  );
}
