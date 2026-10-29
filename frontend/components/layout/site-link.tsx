import Link from "next/link";
import type { ComponentProps } from "react";
import { isStrictPath } from "@/lib/security-headers";

/**
 * A link in the public site's chrome. Links into the signed-in areas load a
 * fresh document, so the stricter Content-Security-Policy of those pages
 * applies (a client-side navigation would keep the public page's policy,
 * D-191). Everything else is a normal Next.js link.
 */
export function SiteLink({ href, prefetch, ...rest }: ComponentProps<typeof Link>) {
  const path = typeof href === "string" ? href.split(/[?#]/)[0] : (href.pathname ?? "");
  if (isStrictPath(path)) {
    return <a href={typeof href === "string" ? href : path} {...rest} />;
  }
  return <Link href={href} prefetch={prefetch} {...rest} />;
}
