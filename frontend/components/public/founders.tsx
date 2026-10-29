import Link from "next/link";
import { ArrowRight } from "@/components/icons";
import { cn } from "@/lib/cn";
import type { PublicFounder } from "@/lib/public/types";
import { ContentImage } from "./content-image";
import { RichText } from "./rich-text";

export const FOUNDERS_PATH = "/about/founders";

function initials(name: string) {
  return name
    .split(/\s+/)
    .filter((w) => /^\p{L}/u.test(w))
    .slice(0, 2)
    .map((w) => w[0]!.toUpperCase())
    .join("");
}

/**
 * The founder's photo, published with their consent. Without one, their
 * initials on navy: a neutral placeholder, never a stand-in face.
 */
export function FounderPortrait({
  founder,
  sizes,
  className,
}: {
  founder: PublicFounder;
  sizes: string;
  className?: string;
}) {
  return (
    <div className={cn("relative", className)}>
      {/* Gold offset frame behind the photo. */}
      <div aria-hidden className="absolute inset-0 translate-x-3 translate-y-3 rounded-card bg-gold/80" />
      {founder.photo ? (
        <ContentImage
          image={founder.photo}
          alt={founder.photo_alt}
          sizes={sizes}
          className="relative aspect-[4/5] w-full rounded-card object-cover shadow-md"
        />
      ) : (
        <div
          aria-hidden
          className="bg-navy-texture relative grid aspect-[4/5] w-full place-items-center rounded-card shadow-md"
        >
          <span className="font-serif text-6xl font-semibold text-gold">{initials(founder.name)}</span>
        </div>
      )}
    </div>
  );
}

/** Home and About: who founded the college, linking to the full stories. */
export function FoundersTeaser({ founders }: { founders: PublicFounder[] }) {
  if (!founders.length) return null;
  const [first] = founders;
  if (founders.length === 1) {
    return (
      <div className="grid items-center gap-10 md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
        <FounderPortrait
          founder={first}
          sizes="(min-width: 768px) 40vw, 90vw"
          className="mx-auto w-full max-w-xs md:max-w-sm"
        />
        <div className="flex flex-col gap-4">
          <div>
            <p className="text-2xl font-semibold text-navy-900 font-serif">{first.name}</p>
            {first.role_title ? <p className="font-semibold text-gold-text">{first.role_title}</p> : null}
          </div>
          {first.intro ? (
            <blockquote className="border-l-4 border-gold pl-4 text-lg">{first.intro}</blockquote>
          ) : null}
          <ReadMore />
        </div>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-6">
      <ul className="grid gap-8 sm:grid-cols-2 lg:grid-cols-3">
        {founders.map((f) => (
          <li key={f.name} className="flex flex-col gap-4">
            <FounderPortrait founder={f} sizes="(min-width: 1024px) 30vw, (min-width: 640px) 45vw, 90vw" />
            <div className="pt-2">
              <p className="font-serif text-xl font-semibold text-navy-900">{f.name}</p>
              {f.role_title ? <p className="font-semibold text-gold-text">{f.role_title}</p> : null}
            </div>
            {f.intro ? <p>{f.intro}</p> : null}
          </li>
        ))}
      </ul>
      <ReadMore />
    </div>
  );
}

function ReadMore() {
  return (
    <Link
      href={FOUNDERS_PATH}
      className="inline-flex min-h-touch items-center gap-1 self-start font-semibold text-royal underline underline-offset-4"
    >
      Read our founders&rsquo; story
      <ArrowRight aria-hidden className="size-4" />
    </Link>
  );
}

/** One founder's full profile on the founders page. */
export function FounderProfile({ founder, reverse }: { founder: PublicFounder; reverse: boolean }) {
  return (
    <article className="grid items-start gap-10 md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
      <FounderPortrait
        founder={founder}
        sizes="(min-width: 768px) 40vw, 90vw"
        className={cn("mx-auto w-full max-w-xs md:sticky md:top-6 md:max-w-sm", reverse && "md:order-last")}
      />
      <div className="flex flex-col gap-4">
        <div>
          <h2 className="text-3xl font-semibold text-navy-900">{founder.name}</h2>
          {founder.role_title ? (
            <p className="text-lg font-semibold text-gold-text">{founder.role_title}</p>
          ) : null}
        </div>
        {founder.intro ? <p className="text-lg">{founder.intro}</p> : null}
        <RichText text={founder.story} headingLevel={3} />
      </div>
    </article>
  );
}
