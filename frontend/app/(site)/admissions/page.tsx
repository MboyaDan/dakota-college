import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "@/components/icons";
import { TextPage, textPageMetadata } from "@/components/public/text-page";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { applyHref } from "@/lib/navigation";

export async function generateMetadata(): Promise<Metadata> {
  return textPageMetadata("admissions", "How to apply", "/admissions");
}

const plan = [
  {
    href: "/admissions/requirements",
    title: "Entry requirements",
    text: "What you need for each programme.",
  },
  { href: "/admissions/fees", title: "Fees", text: "What each programme costs, line by line." },
  { href: "/admissions/intakes", title: "Intake calendar", text: "When programmes start and when to apply." },
  { href: "/faqs", title: "Questions and answers", text: "Answers to what applicants often ask." },
];

export default function AdmissionsPage() {
  return (
    <TextPage slug="admissions" title="How to apply" crumbs={[{ label: "Home", href: "/" }]}>
      <section aria-labelledby="plan-title" className="flex flex-col gap-4">
        <h2 id="plan-title" className="text-2xl font-semibold text-navy-900">
          Plan your application
        </h2>
        <ul className="grid gap-4 sm:grid-cols-2">
          {plan.map((p) => (
            <li key={p.href} className="flex">
              <Card className="relative w-full">
                <h3 className="text-lg font-semibold text-navy-900">
                  <Link href={p.href} className="after:absolute after:inset-0 hover:underline">
                    {p.title}
                  </Link>
                </h3>
                <p>{p.text}</p>
                <ArrowRight aria-hidden className="mt-auto size-5 text-royal" />
              </Card>
            </li>
          ))}
        </ul>
        <div className="flex flex-wrap gap-3">
          <LinkButton href={applyHref}>Apply now</LinkButton>
          <LinkButton href="/programmes" variant="outline">
            Find a course
          </LinkButton>
        </div>
      </section>
    </TextPage>
  );
}
