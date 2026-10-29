import type { Metadata } from "next";
import { connection } from "next/server";

// Student portal (brief 7.1): never indexed.
export const metadata: Metadata = {
  title: {
    template: "%s | Student portal | Dakota Training College",
    default: "Student portal | Dakota Training College",
  },
  robots: { index: false, follow: false },
};

// Rendered per request, with no prerendered shell, so Next.js can put this
// response's script nonce on every script (D-191).
export const instant = false;

export default async function StudentLayout({ children }: LayoutProps<"/student">) {
  await connection();
  return children;
}
