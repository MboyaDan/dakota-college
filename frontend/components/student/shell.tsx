"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import { useStudent } from "./session";

const nav = [
  { href: "/student", label: "Home" },
  { href: "/student/timetable", label: "Timetable" },
  { href: "/student/units", label: "Units" },
  { href: "/student/notices", label: "Notices" },
  { href: "/student/requests", label: "Requests" },
  { href: "/student/profile", label: "Profile" },
];

export function StudentShell({ children }: { children: ReactNode }) {
  const { user, signOut } = useStudent();
  const pathname = usePathname();
  const current = (href: string) =>
    href === "/student" ? pathname === "/student" : pathname.startsWith(href);
  return (
    <div className="flex min-h-full flex-col bg-ivory">
      <a
        href="#student-main"
        className="sr-only z-50 rounded-control bg-gold px-4 py-3 font-semibold text-navy-900 focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
      >
        Skip to main content
      </a>
      <header className="on-dark bg-navy-900 text-white">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 px-4 py-2">
          <Link href="/student" className="flex items-center gap-3 rounded-control">
            <Image
              src="/brand/dakota-logo-reversed.svg"
              alt=""
              width={1339}
              height={442}
              unoptimized
              className="h-12 w-auto"
            />
            <span className="font-serif text-lg font-semibold">Student portal</span>
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
        <nav aria-label="Student portal" className="border-t border-white/15">
          <ul className="mx-auto flex max-w-5xl gap-1 overflow-x-auto px-2 py-1">
            {nav.map((n) => (
              <li key={n.href} className="shrink-0">
                <Link
                  href={n.href}
                  aria-current={current(n.href) ? "page" : undefined}
                  className={cn(
                    "flex min-h-touch items-center rounded-control px-3 whitespace-nowrap",
                    current(n.href) ? "bg-gold font-semibold text-navy-900" : "text-white hover:bg-white/10",
                  )}
                >
                  {n.label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
      </header>
      <main
        id="student-main"
        tabIndex={-1}
        className="mx-auto w-full max-w-5xl min-w-0 flex-1 p-4 focus:outline-none lg:p-8"
      >
        {children}
      </main>
    </div>
  );
}
