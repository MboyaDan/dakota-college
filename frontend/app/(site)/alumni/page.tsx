import type { Metadata } from "next";
import { ContentImage } from "@/components/public/content-image";
import { Section } from "@/components/public/page-header";
import { TextPage, textPageMetadata } from "@/components/public/text-page";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { getSite } from "@/lib/public/site";

export async function generateMetadata(): Promise<Metadata> {
  return textPageMetadata("alumni", "Alumni", "/alumni");
}

export default async function AlumniPage() {
  const site = await getSite();
  return (
    <TextPage slug="alumni" title="Alumni" crumbs={[{ label: "Home", href: "/" }]}>
      {/* Gallery photos ticked for this page; only photos with recorded
          consent can be published (NN-08). Hidden until there are some. */}
      {site.alumni_photos.length ? (
        <Section id="alumni-photos" title="Our alumni" className="-mx-4 lg:-mx-6">
          <ul className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {site.alumni_photos.map((photo) => (
              <li key={photo.image.id}>
                <figure className="flex flex-col gap-2">
                  <ContentImage
                    image={photo.image}
                    alt={photo.alt_text}
                    sizes="(min-width: 1024px) 33vw, (min-width: 640px) 50vw, 100vw"
                    className="aspect-[4/3] w-full rounded-card object-cover"
                  />
                  {photo.caption ? (
                    <figcaption className="text-sm text-ink-muted">{photo.caption}</figcaption>
                  ) : null}
                </figure>
              </li>
            ))}
          </ul>
        </Section>
      ) : null}
      <Card className="max-w-2xl">
        <h2 className="text-2xl font-semibold text-navy-900">Transcripts and certificates</h2>
        <p>
          Former students can ask for a transcript or a copy of a certificate. Send us your request with your
          full name and the year you finished, and the office will reply.
        </p>
        <div>
          <LinkButton href="/contact?about=alumni">Request a transcript or certificate</LinkButton>
        </div>
      </Card>
    </TextPage>
  );
}
