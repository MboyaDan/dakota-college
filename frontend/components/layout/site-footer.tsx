import Image from "next/image";
import Link from "next/link";
import { Clock, Mail, MapPin, Phone } from "@/components/icons";
import { footerNav, legalNav } from "@/lib/navigation";
import { site } from "@/lib/site";

/** Contact rows render only when a real value exists (DQ-01). */
function ContactList() {
  const { phone, email, address, officeHours } = site.contact;
  const rows = [
    phone && { icon: Phone, content: <a href={`tel:${phone.replace(/\s+/g, "")}`}>{phone}</a> },
    email && { icon: Mail, content: <a href={`mailto:${email}`}>{email}</a> },
    address && { icon: MapPin, content: address },
    officeHours && { icon: Clock, content: officeHours },
  ].filter(Boolean) as Array<{ icon: typeof Phone; content: React.ReactNode }>;

  if (rows.length === 0) return null;
  return (
    <ul className="flex flex-col gap-2">
      {rows.map(({ icon: Icon, content }, i) => (
        <li key={i} className="flex items-start gap-2 [&_a]:underline [&_a]:underline-offset-4">
          <Icon aria-hidden className="mt-1 size-4 shrink-0 text-gold" />
          <span>{content}</span>
        </li>
      ))}
    </ul>
  );
}

export function SiteFooter() {
  return (
    <footer className="on-dark mt-auto bg-navy-900 text-white">
      <div className="mx-auto grid max-w-6xl gap-10 px-4 py-12 sm:grid-cols-2 lg:grid-cols-4 lg:px-6">
        <div className="flex flex-col gap-4">
          <Image
            src="/brand/dakota-logo-reversed.svg"
            alt={site.name}
            width={1339}
            height={442}
            unoptimized
            className="-ml-2 h-20 w-auto self-start"
          />
          <p className="text-white/90">{site.location}</p>
          <ContactList />
        </div>
        {footerNav.map((section) => (
          <nav key={section.label} aria-label={section.label} className="flex flex-col gap-3">
            <p className="font-serif text-lg font-semibold text-white">{section.label}</p>
            <ul className="flex flex-col">
              {section.links.map((link) => (
                <li key={link.href}>
                  <Link
                    href={link.href}
                    className="inline-flex min-h-touch items-center text-white/90 hover:text-white hover:underline"
                  >
                    {link.label}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        ))}
      </div>
      <div className="border-t border-white/15">
        <div className="mx-auto flex max-w-6xl flex-col gap-3 px-4 py-6 text-sm text-white/90 lg:flex-row lg:items-center lg:justify-between lg:px-6">
          <div className="flex flex-col gap-1">
            <p>© {site.name}</p>
            {site.registration ? <p>{site.registration}</p> : null}
          </div>
          <nav aria-label="Legal">
            <ul className="flex flex-wrap gap-x-4">
              {legalNav.map((link) => (
                <li key={link.href}>
                  <Link
                    href={link.href}
                    className="inline-flex min-h-touch items-center hover:text-white hover:underline"
                  >
                    {link.label}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        </div>
      </div>
    </footer>
  );
}
