import type { Metadata } from "next";
import Image from "next/image";
import { notFound } from "next/navigation";
import type { ReactNode } from "react";
import { ToastDemo } from "@/components/design/toast-demo";
import * as Icons from "@/components/icons";
import { Accordion, AccordionItem } from "@/components/ui/accordion";
import { Alert } from "@/components/ui/alert";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Button, LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Checkbox, Input, Select, Textarea } from "@/components/ui/field";
import { Pagination } from "@/components/ui/pagination";
import { Loading, Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { Table } from "@/components/ui/table";
import { Tabs } from "@/components/ui/tabs";
import { StatusTag, Tag } from "@/components/ui/tag";
import { contrastRatio, tokens, usedPairs } from "@/lib/contrast";
import { schoolAccent, schools } from "@/lib/schools";

// Living style guide (Stage 2). Never indexed, and not built into production
// unless ENABLE_DESIGN_PAGE=1 (CI sets it so axe can check every component).
export const metadata: Metadata = {
  title: "Design system",
  robots: { index: false, follow: false },
};

const designPageEnabled = process.env.NODE_ENV !== "production" || process.env.ENABLE_DESIGN_PAGE === "1";

function Section({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="flex flex-col gap-4 border-t border-stone pt-8">
      <h2 id={id} className="text-2xl font-semibold">
        {title}
      </h2>
      {children}
    </section>
  );
}

const uiIcons = [
  "ArrowRight",
  "Calendar",
  "Check",
  "ChevronDown",
  "CircleAlert",
  "CircleCheck",
  "CircleX",
  "Clock",
  "ExternalLink",
  "GraduationCap",
  "Info",
  "Lock",
  "Mail",
  "MapPin",
  "Megaphone",
  "Menu",
  "MessageCircle",
  "Phone",
  "Search",
  "User",
] as const;

type SampleRow = { id: string; item: string; amount: string };
const sampleRows: SampleRow[] = [
  { id: "1", item: "Sample line A", amount: "KES 1,000.00" },
  { id: "2", item: "Sample line B", amount: "KES 25,500.00" },
];

export default function DesignPage() {
  if (!designPageEnabled) notFound();

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-10 px-4 py-10 lg:px-6">
      <header className="flex flex-col gap-2">
        <Breadcrumb items={[{ label: "Home", href: "/" }, { label: "Design system" }]} />
        <h1 className="text-4xl font-semibold">Design system</h1>
        <p className="max-w-prose text-ink-muted">
          Every component, token and icon in one place. All text here is sample text for development.
        </p>
      </header>

      <Section id="colours" title="Colours and measured contrast">
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {Object.entries(tokens).map(([name, hex]) => (
            <li key={name} className="flex items-center gap-3">
              <span
                aria-hidden
                className="size-10 shrink-0 rounded-control border border-stone"
                style={{ background: hex }}
              />
              <span className="flex flex-col text-sm">
                <span className="font-semibold">{name}</span>
                <span className="font-mono">{hex}</span>
              </span>
            </li>
          ))}
        </ul>
        <Table
          caption="Text and background pairs used in the interface (WCAG 2 contrast)"
          rowKey={(p) => `${p.fg}-${p.bg}-${p.use}`}
          rows={usedPairs}
          columns={[
            { key: "use", header: "Used for", cell: (p) => p.use },
            {
              key: "sample",
              header: "Sample",
              cell: (p) => (
                <span
                  className="rounded px-2 py-1 font-semibold"
                  style={{ color: tokens[p.fg], background: tokens[p.bg] }}
                >
                  Aa
                </span>
              ),
            },
            {
              key: "ratio",
              header: "Ratio",
              numeric: true,
              cell: (p) => `${contrastRatio(tokens[p.fg], tokens[p.bg]).toFixed(1)}:1`,
            },
            { key: "min", header: "Needs", numeric: true, cell: (p) => `${p.min}:1` },
          ]}
        />
      </Section>

      <Section id="type" title="Typography">
        <p className="font-serif text-5xl font-semibold text-navy-900">Newsreader heading</p>
        <p className="font-serif text-3xl font-semibold text-navy-900">Newsreader subheading</p>
        <p className="max-w-prose">
          Inter body text at 17 px. Short sentences and plain words.{" "}
          <a href="#type" className="text-royal underline underline-offset-4">
            A link looks like this.
          </a>
        </p>
        <p className="text-gold-text font-semibold">Gold-coloured text uses the darker gold-text token.</p>
        <p>
          Application reference: <span className="font-mono">DTC-APP-0000-SAMPLE</span>
        </p>
        <p className="tabular">Tabular figures: KES 1,250.00 / KES 98,000.00</p>
      </Section>

      <Section id="logos" title="Logo files (as supplied)">
        <div className="grid gap-4 sm:grid-cols-2">
          <Card>
            <Image
              src="/brand/dakota-logo-horizontal.svg"
              alt="Dakota Training College logo, horizontal"
              width={1339}
              height={442}
              unoptimized
              className="h-auto w-full"
            />
          </Card>
          <Card className="bg-navy-900">
            <Image
              src="/brand/dakota-logo-reversed.svg"
              alt="Dakota Training College logo, reversed for dark backgrounds"
              width={1339}
              height={442}
              unoptimized
              className="h-auto w-full"
            />
          </Card>
          <Card>
            <Image
              src="/brand/dakota-logo-stacked-tagline-progress.svg"
              alt="Dakota Training College logo, stacked, with the tagline Turning Potential into Progress"
              width={916}
              height={858}
              unoptimized
              className="mx-auto h-56 w-auto"
            />
          </Card>
          <Card className="justify-center">
            <Image
              src="/brand/dakota-ribbon-banner.svg"
              alt="Skills, Discipline, Opportunity"
              width={3646}
              height={330}
              unoptimized
              className="h-auto w-full"
            />
          </Card>
        </div>
      </Section>

      <Section id="icons" title="Icons">
        <h3 className="text-xl font-semibold">Schools</h3>
        <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {schools.map((s) => (
            <li key={s.key} className="flex items-center gap-3">
              <span
                className={`inline-flex size-11 items-center justify-center rounded-full text-white ${schoolAccent[s.key].bg}`}
              >
                <s.icon aria-hidden className="size-6" />
              </span>
              <span>{s.name}</span>
            </li>
          ))}
        </ul>
        <h3 className="text-xl font-semibold">Interface</h3>
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-5">
          {uiIcons.map((name) => {
            const Icon = Icons[name];
            return (
              <li key={name} className="flex items-center gap-2 text-sm">
                <Icon aria-hidden className="size-5 text-navy-900" />
                {name}
              </li>
            );
          })}
        </ul>
      </Section>

      <Section id="buttons" title="Buttons">
        <div className="flex flex-wrap gap-3">
          <LinkButton href="/apply">Apply now</LinkButton>
          <Button variant="secondary">Secondary</Button>
          <Button variant="outline">Outline</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="danger">Delete</Button>
          <Button loading>Saving</Button>
          <Button disabled>Disabled</Button>
        </div>
        <div className="on-dark flex flex-wrap gap-3 rounded-card bg-navy-900 p-4">
          <LinkButton href="/apply">Apply now</LinkButton>
          <Button variant="inverse">On navy</Button>
        </div>
      </Section>

      <Section id="forms" title="Form fields">
        <form className="grid max-w-xl gap-5" action="#forms">
          <Input name="demo-name" label="Full name" required autoComplete="name" />
          <Input
            name="demo-phone"
            label="Phone number"
            type="tel"
            hint="We will only call about your enquiry."
            autoComplete="tel"
          />
          <Input
            name="demo-email"
            label="Email address"
            type="email"
            required
            defaultValue="not-an-email"
            error="Enter an email address in the format name@example.com."
          />
          <Select name="demo-topic" label="What is your question about?" required defaultValue="">
            <option value="" disabled>
              Choose a topic
            </option>
            <option>Admissions</option>
            <option>Fees</option>
            <option>Other</option>
          </Select>
          <Textarea name="demo-message" label="Your message" hint="Up to 1,000 characters." />
          <Checkbox
            name="demo-consent"
            label="I agree that the college may contact me about this enquiry."
            hint="Sample consent text."
          />
        </form>
      </Section>

      <Section id="cards" title="Cards and tags">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {schools.map((s) => (
            <Card key={s.key} accent={s.key}>
              <Tag school={s.key}>{s.shortName}</Tag>
              <p className="font-serif text-xl font-semibold text-navy-900">Sample programme card</p>
              <p className="text-ink-muted">Card content goes here.</p>
            </Card>
          ))}
        </div>
        <div className="flex flex-wrap gap-2">
          <Tag>Neutral</Tag>
          <Tag tone="navy">Navy</Tag>
          <Tag tone="gold">Gold</Tag>
          <StatusTag status="success">Approved</StatusTag>
          <StatusTag status="danger">Rejected</StatusTag>
          <StatusTag status="warning">Information needed</StatusTag>
          <StatusTag status="info">Under review</StatusTag>
        </div>
      </Section>

      <Section id="alerts" title="Alerts">
        <Alert tone="info" title="Applications are saved as you go">
          You can close this page and continue later with your reference number.
        </Alert>
        <Alert tone="success" title="Message sent" />
        <Alert tone="warning" title="Some details are missing">
          Add your guardian&apos;s phone number before you submit.
        </Alert>
        <Alert tone="danger" title="We could not upload your file">
          Choose a PDF, JPG or PNG under 5 MB and try again.
        </Alert>
      </Section>

      <Section id="tabs" title="Tabs">
        <Tabs
          label="Sample levels"
          items={[
            { id: "diploma", label: "Diploma", content: <p>Sample diploma panel.</p> },
            { id: "certificate", label: "Certificate", content: <p>Sample certificate panel.</p> },
            { id: "short", label: "Short courses", content: <p>Sample short course panel.</p> },
          ]}
        />
      </Section>

      <Section id="accordion" title="Accordion">
        <Accordion>
          <AccordionItem title="Sample question one" defaultOpen>
            Sample answer. Accordions use native details and summary elements.
          </AccordionItem>
          <AccordionItem title="Sample question two">Another sample answer.</AccordionItem>
        </Accordion>
      </Section>

      <Section id="dialog" title="Dialog and toast">
        <div className="flex flex-wrap gap-3">
          <Dialog
            title="Withdraw application?"
            description="You can start a new application later."
            triggerLabel="Open dialog"
            actions={
              <>
                <Button type="submit" variant="outline">
                  Keep application
                </Button>
                <Button type="submit" variant="danger">
                  Withdraw
                </Button>
              </>
            }
          >
            <p>Sample dialog body.</p>
          </Dialog>
          <ToastDemo />
        </div>
      </Section>

      <Section id="table" title="Table">
        <Table
          caption="Sample statement"
          rowKey={(r) => r.id}
          rows={sampleRows}
          columns={[
            { key: "item", header: "Item", cell: (r) => r.item },
            { key: "amount", header: "Amount", numeric: true, cell: (r) => r.amount },
          ]}
        />
        <Table
          caption="Empty table"
          rowKey={(r: SampleRow) => r.id}
          rows={[]}
          columns={[{ key: "item", header: "Item", cell: (r) => r.item }]}
          empty="No payments recorded yet."
        />
      </Section>

      <Section id="navigation" title="Pagination and breadcrumb">
        <Pagination
          page={4}
          totalPages={12}
          hrefFor={(p) => `/design?page=${p}#navigation`}
          label="Sample pages"
        />
        <Breadcrumb
          items={[
            { label: "Home", href: "/" },
            { label: "Programmes", href: "/programmes" },
            { label: "Sample programme" },
          ]}
        />
      </Section>

      <Section id="states" title="Loading and empty states">
        <Loading label="Loading programmes">
          <div className="grid gap-4 sm:grid-cols-2">
            {[0, 1].map((i) => (
              <Card key={i}>
                <Skeleton className="h-6 w-1/2" />
                <SkeletonText />
              </Card>
            ))}
          </div>
        </Loading>
        <EmptyState
          icon={Icons.Search}
          title="No programmes match your search"
          description="Try fewer filters, or search for a school name instead."
          action={
            <LinkButton href="/programmes" variant="outline">
              Clear filters
            </LinkButton>
          }
        />
      </Section>
    </div>
  );
}
