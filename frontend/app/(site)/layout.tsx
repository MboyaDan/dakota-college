import { SiteFooter } from "@/components/layout/site-footer";
import { SiteHeader } from "@/components/layout/site-header";
import { AnnouncementBar, SkipLink, UtilityBar, type Announcement } from "@/components/layout/top-bars";
import { WhatsAppButton } from "@/components/public/whatsapp-button";
import { utilityNav } from "@/lib/navigation";
import { settingsOf } from "@/lib/public/settings";
import { getSite } from "@/lib/public/site";

/** Public website shell: announcement, utility bar, header, footer. */
export default async function SiteLayout({ children }: LayoutProps<"/">) {
  const site = await getSite();
  const settings = settingsOf(site);
  // The API returns only banners active now; the newest first.
  const banner = site.banners[0];
  const announcement: Announcement | null = banner
    ? {
        message: banner.message,
        link:
          banner.link_href && banner.link_label
            ? { href: banner.link_href, label: banner.link_label }
            : undefined,
      }
    : null;

  return (
    <>
      <SkipLink />
      <AnnouncementBar announcement={announcement} />
      <UtilityBar links={utilityNav} />
      <SiteHeader />
      <main id="main" tabIndex={-1} className="flex flex-1 flex-col focus:outline-none">
        {children}
      </main>
      <SiteFooter settings={settings} accreditations={site.accreditations} />
      <WhatsAppButton number={settings.whatsapp} officeHours={settings.officeHours} />
    </>
  );
}
