import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { Search } from "@/components/icons";
import { PageHeader } from "@/components/public/page-header";
import { Accordion, AccordionItem } from "@/components/ui/accordion";
import { buttonClasses } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { groupFaqs, searchFaqs } from "@/lib/public/faqs";
import { getSiteOrFallback } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";

export const metadata: Metadata = pageMetadata({
  title: "Questions and answers",
  description:
    "Answers to common questions about applying, fees and student life at Dakota Training College.",
  path: "/faqs",
});

export default function FaqsPage({ searchParams }: PageProps<"/faqs">) {
  return (
    <>
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "Questions and answers" }]}
        title="Questions and answers"
        lead="Search the questions applicants and students often ask, or browse by topic."
      />
      <div className="mx-auto flex w-full max-w-4xl flex-col gap-8 px-4 py-8 lg:px-6">
        <Suspense
          fallback={
            <Loading label="Loading questions">
              <Skeleton className="h-24 w-full" />
              <Skeleton className="mt-4 h-64 w-full" />
            </Loading>
          }
        >
          <Faqs searchParams={searchParams} />
        </Suspense>
      </div>
    </>
  );
}

async function Faqs({ searchParams }: Pick<PageProps<"/faqs">, "searchParams">) {
  const [site, params] = await Promise.all([getSiteOrFallback(), searchParams]);
  const q = (Array.isArray(params.q) ? params.q[0] : params.q)?.trim().slice(0, 100) ?? "";
  const titles = new Map(site.programmes.map((p) => [p.slug, p.title]));
  const groups = groupFaqs(searchFaqs(site.faqs, q));
  const count = groups.reduce((n, g) => n + g.items.length, 0);

  if (!site.faqs.length) {
    return (
      <EmptyState
        title="Questions are on their way"
        description="The college is writing answers to common questions. Contact us with yours in the meantime."
      />
    );
  }

  return (
    <>
      <form
        action="/faqs"
        role="search"
        aria-label="Search questions"
        className="flex flex-col gap-2 sm:flex-row sm:items-end"
      >
        <div className="flex flex-1 flex-col gap-1.5">
          <label htmlFor="faq-q" className="font-semibold">
            Search questions
          </label>
          <input
            id="faq-q"
            name="q"
            type="search"
            defaultValue={q}
            placeholder="For example fees or accommodation"
            className="block min-h-touch w-full rounded-control border border-ink-muted bg-white px-3 py-2 text-base"
          />
        </div>
        <button type="submit" className={buttonClasses("primary")}>
          <Search aria-hidden className="size-5" />
          Search
        </button>
      </form>

      {q ? (
        <p role="status" className="flex flex-wrap items-baseline gap-x-3">
          <span className="font-semibold">
            {count === 1 ? "1 question matches" : `${count} questions match`} “{q}”.
          </span>
          <Link href="/faqs" className="inline-flex min-h-touch items-center text-royal underline">
            Show all questions
          </Link>
        </p>
      ) : (
        <nav aria-label="Topics">
          <ul className="flex flex-wrap gap-x-4">
            {groups.map((g) => (
              <li key={g.slug}>
                <a
                  href={`#${g.slug}`}
                  className="inline-flex min-h-touch items-center font-semibold text-royal underline"
                >
                  {g.name}
                </a>
              </li>
            ))}
          </ul>
        </nav>
      )}

      {groups.length ? (
        groups.map((g) => (
          <section
            key={g.slug}
            id={g.slug}
            aria-labelledby={`${g.slug}-title`}
            className="flex scroll-mt-4 flex-col gap-3"
          >
            <h2 id={`${g.slug}-title`} className="text-2xl font-semibold text-navy-900">
              {g.name}
            </h2>
            <Accordion>
              {g.items.map((f) => (
                <AccordionItem key={f.question} title={f.question} defaultOpen={Boolean(q)}>
                  {f.programme_slug && titles.get(f.programme_slug) ? (
                    <p className="mb-2 text-sm text-ink-muted">
                      About{" "}
                      <Link href={`/programmes/${f.programme_slug}`} className="text-royal underline">
                        {titles.get(f.programme_slug)}
                      </Link>
                    </p>
                  ) : null}
                  <p className="whitespace-pre-line">{f.answer}</p>
                </AccordionItem>
              ))}
            </Accordion>
          </section>
        ))
      ) : (
        <EmptyState
          title="No questions match"
          description="Try a different word, or ask us directly."
          action={
            <Link href="/contact" className="font-semibold text-royal underline">
              Ask a question
            </Link>
          }
        />
      )}
    </>
  );
}
