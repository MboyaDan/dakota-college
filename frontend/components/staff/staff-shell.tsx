"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import { kindOrder, kinds } from "@/lib/staff/kinds";
import { useSession } from "./session";

type NavItem = { href: string; label: string; show: boolean };

export function StaffShell({ children }: { children: ReactNode }) {
  const { user, can, signOut } = useSession();
  const pathname = usePathname();

  const nav: NavItem[] = [
    { href: "/staff", label: "Overview", show: true },
    { href: "/staff/applications", label: "Applications", show: can("admissions.view") },
    { href: "/staff/students", label: "Students", show: can("students.view") },
    { href: "/staff/student-requests", label: "Student requests", show: can("students.view") },
    { href: "/staff/teaching", label: "My teaching", show: can("academics.teach") },
    { href: "/staff/offerings", label: "Classes", show: can("academics.manage") },
    { href: "/staff/academics", label: "Units and terms", show: can("academics.manage") },
    { href: "/staff/notices", label: "Notices", show: can("academics.manage") },
    { href: "/staff/enquiries", label: "Enquiries", show: can("enquiries.view") },
    { href: "/staff/approvals", label: "Waiting for approval", show: can("content.publish") },
    ...kindOrder.map((k) => ({
      href: `/staff/content/${k}`,
      label: kinds[k].title,
      show: can("content.read"),
    })),
    { href: "/staff/settings", label: "Site settings", show: can("content.read") },
    { href: "/staff/messages", label: "Messages to applicants", show: can("notifications.manage") },
    { href: "/staff/staff-schools", label: "Staff schools", show: can("users.manage") },
  ];
  const isCurrent = (href: string) => (href === "/staff" ? pathname === "/staff" : pathname.startsWith(href));

  return (
    <div className="flex min-h-full flex-col bg-ivory">
      <a
        href="#staff-main"
        className="sr-only z-50 rounded-control bg-gold px-4 py-3 font-semibold text-navy-900 focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
      >
        Skip to main content
      </a>
      <header className="on-dark bg-navy-900 text-white">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-2 lg:px-6">
          <Link href="/staff" className="flex items-center gap-3 rounded-control">
            <Image
              src="/brand/dakota-logo-reversed.svg"
              alt=""
              width={1339}
              height={442}
              unoptimized
              className="h-12 w-auto"
            />
            <span className="font-serif text-lg font-semibold">Staff portal</span>
          </Link>
          <div className="flex items-center gap-3 text-sm">
            <span className="hidden sm:inline">{user.full_name}</span>
            <button
              type="button"
              onClick={signOut}
              className="inline-flex min-h-touch items-center rounded-control border-2 border-white px-3 font-semibold hover:bg-white/10"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>
      <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col lg:flex-row">
        <nav
          aria-label="Staff"
          className="border-b border-stone bg-white lg:w-60 lg:shrink-0 lg:border-r lg:border-b-0"
        >
          <ul className="flex gap-1 overflow-x-auto p-2 lg:flex-col">
            {nav
              .filter((n) => n.show)
              .map((n) => (
                <li key={n.href} className="shrink-0">
                  <Link
                    href={n.href}
                    aria-current={isCurrent(n.href) ? "page" : undefined}
                    className={cn(
                      "flex min-h-touch items-center rounded-control px-3 whitespace-nowrap",
                      isCurrent(n.href)
                        ? "bg-navy-900 font-semibold text-white"
                        : "text-royal hover:bg-ivory",
                    )}
                  >
                    {n.label}
                  </Link>
                </li>
              ))}
          </ul>
        </nav>
        <main id="staff-main" tabIndex={-1} className="min-w-0 flex-1 p-4 focus:outline-none lg:p-8">
          {children}
        </main>
      </div>
    </div>
  );
}
