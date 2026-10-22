import { SiteChrome } from "@/components/layout/site-chrome";

export default function SiteLayout({ children }: LayoutProps<"/">) {
  return <SiteChrome>{children}</SiteChrome>;
}
