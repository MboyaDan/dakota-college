"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useId, useRef, useState } from "react";
import { ArrowRight, ChevronDown, Menu, X } from "@/components/icons";
import { buttonClasses } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import type { NavItem, NavLink } from "@/lib/navigation";

type MainNavProps = { items: NavItem[]; utility: NavLink[]; applyHref: string };

/**
 * Desktop (lg and up): a bar of links and mega-menu buttons. Menus open on
 * click or Enter, close on Escape (focus returns to the button), outside
 * click or navigation. Mobile: a "Menu" button that reveals the same links as
 * collapsible sections, plus the utility links and Apply.
 */
export function MainNav({ items, utility, applyHref }: MainNavProps) {
  const pathname = usePathname();
  const navRef = useRef<HTMLDivElement>(null);
  const buttonRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const baseId = useId();

  const [openMenu, setOpenMenu] = useState<string | null>(null);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [mobileSection, setMobileSection] = useState<string | null>(null);
  const [lastPath, setLastPath] = useState(pathname);

  // Close everything after navigation (state adjusted during render, not in an effect).
  if (pathname !== lastPath) {
    setLastPath(pathname);
    setOpenMenu(null);
    setMobileOpen(false);
  }

  useEffect(() => {
    if (!openMenu && !mobileOpen) return;
    function onKey(e: KeyboardEvent) {
      if (e.key !== "Escape") return;
      if (openMenu) buttonRefs.current[openMenu]?.focus();
      if (mobileOpen) buttonRefs.current.__mobile?.focus();
      setOpenMenu(null);
      setMobileOpen(false);
    }
    function onPointer(e: PointerEvent) {
      if (navRef.current && !navRef.current.contains(e.target as Node)) {
        setOpenMenu(null);
        setMobileOpen(false);
      }
    }
    document.addEventListener("keydown", onKey);
    document.addEventListener("pointerdown", onPointer);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("pointerdown", onPointer);
    };
  }, [openMenu, mobileOpen]);

  const panelId = (label: string) => `${baseId}-${label.replace(/\W+/g, "-").toLowerCase()}`;
  const isCurrent = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));
  // Utility links already in the main list (e.g. Contact) are not repeated in the mobile menu.
  const mainHrefs = new Set(items.flatMap((i) => (i.kind === "link" ? [i.href] : [])));
  const mobileUtility = utility.filter((l) => !mainHrefs.has(l.href));

  return (
    <div ref={navRef} className="flex items-center gap-2">
      {/* Desktop */}
      <nav aria-label="Main" className="hidden lg:block">
        <ul className="flex items-center gap-1">
          {items.map((item) =>
            item.kind === "link" ? (
              <li key={item.label}>
                <Link
                  href={item.href}
                  aria-current={isCurrent(item.href) ? "page" : undefined}
                  className="inline-flex min-h-touch items-center rounded-control px-3 font-medium text-white hover:bg-white/10 aria-[current=page]:underline aria-[current=page]:decoration-gold aria-[current=page]:decoration-2 aria-[current=page]:underline-offset-8"
                >
                  {item.label}
                </Link>
              </li>
            ) : (
              <li key={item.label}>
                <button
                  ref={(el) => {
                    buttonRefs.current[item.label] = el;
                  }}
                  type="button"
                  aria-expanded={openMenu === item.label}
                  aria-controls={panelId(item.label)}
                  onClick={() => setOpenMenu((m) => (m === item.label ? null : item.label))}
                  className="inline-flex min-h-touch items-center gap-1 rounded-control px-3 font-medium text-white hover:bg-white/10 aria-expanded:bg-white/10"
                >
                  {item.label}
                  <ChevronDown
                    aria-hidden
                    className={cn("size-4 transition-transform", openMenu === item.label && "rotate-180")}
                  />
                </button>
                <div
                  id={panelId(item.label)}
                  hidden={openMenu !== item.label}
                  className="absolute inset-x-0 top-full z-40 border-t-4 border-gold bg-white text-ink shadow-xl"
                >
                  <div className="mx-auto grid max-w-6xl gap-8 px-6 py-8 sm:grid-cols-2 lg:grid-cols-3">
                    {item.sections.map((section) => (
                      <div key={section.label} className="flex flex-col gap-2">
                        <p className="font-serif text-lg font-semibold text-navy-900">{section.label}</p>
                        <ul className="flex flex-col">
                          {section.links.map((link) => (
                            <li key={link.href}>
                              <Link
                                href={link.href}
                                className="flex min-h-touch flex-col justify-center rounded-control px-2 py-1 hover:bg-ivory"
                              >
                                <span className="font-medium text-royal">{link.label}</span>
                                {link.description ? (
                                  <span className="text-sm text-ink-muted">{link.description}</span>
                                ) : null}
                              </Link>
                            </li>
                          ))}
                        </ul>
                      </div>
                    ))}
                    {item.footerLink ? (
                      <div className="flex items-end">
                        <Link
                          href={item.footerLink.href}
                          className="inline-flex min-h-touch items-center gap-2 font-semibold text-royal underline underline-offset-4"
                        >
                          {item.footerLink.label}
                          <ArrowRight aria-hidden className="size-4" />
                        </Link>
                      </div>
                    ) : null}
                  </div>
                </div>
              </li>
            ),
          )}
        </ul>
      </nav>

      <Link href={applyHref} className={buttonClasses("primary", "px-4")}>
        Apply now
      </Link>

      {/* Mobile */}
      <button
        ref={(el) => {
          buttonRefs.current.__mobile = el;
        }}
        type="button"
        aria-expanded={mobileOpen}
        aria-controls={`${baseId}-mobile`}
        onClick={() => setMobileOpen((o) => !o)}
        className="inline-flex min-h-touch min-w-touch items-center justify-center gap-2 rounded-control px-2 font-medium text-white hover:bg-white/10 lg:hidden"
      >
        {mobileOpen ? <X aria-hidden className="size-6" /> : <Menu aria-hidden className="size-6" />}
        <span className="sr-only sm:not-sr-only">Menu</span>
      </button>

      <nav
        id={`${baseId}-mobile`}
        aria-label="Main"
        hidden={!mobileOpen}
        className="absolute inset-x-0 top-full z-40 max-h-[calc(100dvh-4rem)] overflow-y-auto border-t-4 border-gold bg-white text-ink shadow-xl lg:hidden"
      >
        <ul className="flex flex-col divide-y divide-stone px-4 py-2">
          {items.map((item) =>
            item.kind === "link" ? (
              <li key={item.label}>
                <Link
                  href={item.href}
                  className="flex min-h-touch items-center py-2 font-semibold text-navy-900"
                >
                  {item.label}
                </Link>
              </li>
            ) : (
              <li key={item.label}>
                <button
                  type="button"
                  aria-expanded={mobileSection === item.label}
                  aria-controls={`${panelId(item.label)}-m`}
                  onClick={() => setMobileSection((s) => (s === item.label ? null : item.label))}
                  className="flex min-h-touch w-full items-center justify-between py-2 text-left font-semibold text-navy-900"
                >
                  {item.label}
                  <ChevronDown
                    aria-hidden
                    className={cn(
                      "size-5 transition-transform",
                      mobileSection === item.label && "rotate-180",
                    )}
                  />
                </button>
                <div id={`${panelId(item.label)}-m`} hidden={mobileSection !== item.label} className="pb-3">
                  {item.sections.map((section) => (
                    <div key={section.label} className="pt-2">
                      <p className="text-sm font-semibold tracking-wide text-ink-muted uppercase">
                        {section.label}
                      </p>
                      <ul>
                        {section.links.map((link) => (
                          <li key={link.href}>
                            <Link href={link.href} className="flex min-h-touch items-center pl-3 text-royal">
                              {link.label}
                            </Link>
                          </li>
                        ))}
                      </ul>
                    </div>
                  ))}
                </div>
              </li>
            ),
          )}
          <li className="py-3">
            <ul className="grid grid-cols-2 gap-x-4">
              {mobileUtility.map((link) => (
                <li key={link.href}>
                  <Link href={link.href} className="flex min-h-touch items-center text-royal">
                    {link.label}
                  </Link>
                </li>
              ))}
            </ul>
          </li>
        </ul>
      </nav>
    </div>
  );
}
