import Image from "next/image";
import Link from "next/link";
import { applyHref, mainNav, utilityNav } from "@/lib/navigation";
import { site } from "@/lib/site";
import { MainNav } from "./main-nav";

/** Navy header band: logo, main navigation with mega-menus, Apply now. */
export function SiteHeader() {
  return (
    <header className="on-dark relative bg-navy-900 text-white">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-2 px-4 lg:h-20 lg:px-6">
        <Link href="/" className="shrink-0 rounded-control">
          {/* Supplied logo, unchanged. Its navy background matches the header. */}
          <Image
            src="/brand/dakota-logo-reversed.svg"
            alt={`${site.name} home`}
            width={890}
            height={330}
            unoptimized
            priority
            className="h-12 w-auto sm:h-14 lg:h-[4.5rem]"
          />
        </Link>
        <MainNav items={mainNav} utility={utilityNav} applyHref={applyHref} />
      </div>
    </header>
  );
}
