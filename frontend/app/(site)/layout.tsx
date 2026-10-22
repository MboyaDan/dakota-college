import { SiteFooter } from "@/components/layout/site-footer";
import { SiteHeader } from "@/components/layout/site-header";
import { AnnouncementBar, SkipLink, UtilityBar } from "@/components/layout/top-bars";
import { utilityNav } from "@/lib/navigation";

/** Public website shell: announcement, utility bar, header, footer. */
export default function SiteLayout({ children }: LayoutProps<"/">) {
  return (
    <>
      <SkipLink />
      {/* Announcements come from the content API from Stage 6. */}
      <AnnouncementBar announcement={null} />
      <UtilityBar links={utilityNav} />
      <SiteHeader />
      <main id="main" tabIndex={-1} className="flex flex-1 flex-col focus:outline-none">
        {children}
      </main>
      <SiteFooter />
    </>
  );
}
