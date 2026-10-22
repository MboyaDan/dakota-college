import type { Metadata } from "next";
import { ContentImage } from "@/components/public/content-image";
import { Section } from "@/components/public/page-header";
import { TextPage, textPageMetadata } from "@/components/public/text-page";
import { getSite } from "@/lib/public/site";

export async function generateMetadata(): Promise<Metadata> {
  return textPageMetadata("student-life", "Student life", "/student-life");
}

export default async function StudentLifePage() {
  const site = await getSite();
  return (
    <TextPage slug="student-life" title="Student life" crumbs={[{ label: "Home", href: "/" }]}>
      {/* Only photos with recorded consent can be published (NN-08). */}
      {site.gallery.length ? (
        <Section id="gallery" title="Life at Dakota" className="-mx-4 lg:-mx-6">
          <ul className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {site.gallery.map((photo) => (
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
    </TextPage>
  );
}
