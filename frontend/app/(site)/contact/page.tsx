import type { Metadata } from "next";
import type { ReactNode } from "react";
import { Clock, ExternalLink, Mail, MapPin, MessageCircle, Phone } from "@/components/icons";
import { ContactForm } from "@/components/public/contact-form";
import { PageHeader } from "@/components/public/page-header";
import { Card } from "@/components/ui/card";
import { settingsOf, telHref, whatsappHref } from "@/lib/public/settings";
import { getSite } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";

export const metadata: Metadata = pageMetadata({
  title: "Contact us",
  description:
    "Ask Dakota Training College about programmes, admissions or fees, by message, phone, email or WhatsApp.",
  path: "/contact",
});

function Row({ icon, label, children }: { icon: ReactNode; label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="flex items-center gap-2 text-sm text-ink-muted">
        <span className="text-royal">{icon}</span>
        {label}
      </dt>
      <dd className="pl-7 font-semibold [&_a]:text-royal [&_a]:underline [&_a]:underline-offset-4">
        {children}
      </dd>
    </div>
  );
}

export default async function ContactPage() {
  const site = await getSite();
  const s = settingsOf(site);
  const whatsapp = whatsappHref(s.whatsapp, "Hello Dakota Training College, I have a question:");
  const hasDetails = s.phone || s.email || s.admissionsEmail || whatsapp || s.address || s.officeHours;

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "Contact" }]}
        title="Contact us"
        lead="Send us your question and the right office will reply."
      />
      <div className="mx-auto grid w-full max-w-6xl gap-8 px-4 py-10 lg:grid-cols-[1.4fr_1fr] lg:px-6">
        <section aria-labelledby="form-title" className="relative flex flex-col gap-4">
          <h2 id="form-title" className="text-2xl font-semibold text-navy-900">
            Send a message
          </h2>
          <Card>
            <ContactForm
              programmes={site.programmes.map((p) => ({ slug: p.slug, title: p.title }))}
              fallbackContact={s.phone || s.email}
            />
          </Card>
        </section>

        {hasDetails ? (
          <section aria-labelledby="details-title" className="flex flex-col gap-4">
            <h2 id="details-title" className="text-2xl font-semibold text-navy-900">
              Other ways to reach us
            </h2>
            <Card>
              <dl className="flex flex-col gap-4">
                {s.phone ? (
                  <Row icon={<Phone aria-hidden className="size-5" />} label="Phone">
                    <a href={telHref(s.phone)}>{s.phone}</a>
                  </Row>
                ) : null}
                {whatsapp ? (
                  <Row icon={<MessageCircle aria-hidden className="size-5" />} label="WhatsApp">
                    <a href={whatsapp} target="_blank" rel="noopener noreferrer">
                      Chat on WhatsApp <span className="sr-only">(opens WhatsApp)</span>
                    </a>
                  </Row>
                ) : null}
                {s.admissionsEmail ? (
                  <Row icon={<Mail aria-hidden className="size-5" />} label="Admissions email">
                    <a href={`mailto:${s.admissionsEmail}`}>{s.admissionsEmail}</a>
                  </Row>
                ) : null}
                {s.email ? (
                  <Row icon={<Mail aria-hidden className="size-5" />} label="General email">
                    <a href={`mailto:${s.email}`}>{s.email}</a>
                  </Row>
                ) : null}
                {s.officeHours ? (
                  <Row icon={<Clock aria-hidden className="size-5" />} label="Office hours">
                    {s.officeHours}
                  </Row>
                ) : null}
                {s.address ? (
                  <Row icon={<MapPin aria-hidden className="size-5" />} label="Address">
                    {s.address}
                    {s.mapUrl ? (
                      <>
                        <br />
                        <a
                          href={s.mapUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1"
                        >
                          Get directions <ExternalLink aria-hidden className="size-4" />
                          <span className="sr-only">(opens a map)</span>
                        </a>
                      </>
                    ) : null}
                  </Row>
                ) : null}
              </dl>
            </Card>
          </section>
        ) : null}
      </div>
    </>
  );
}
