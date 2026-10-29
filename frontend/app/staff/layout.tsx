import type { Metadata } from "next";
import { connection } from "next/server";

// Staff portal: never indexed (brief 12.1).
export const metadata: Metadata = {
  title: {
    template: "%s | Staff portal | Dakota Training College",
    default: "Staff portal | Dakota Training College",
  },
  robots: { index: false, follow: false },
};

// Rendered per request, with no prerendered shell, so Next.js can put this
// response's script nonce on every script (D-191).
export const instant = false;

export default async function StaffLayout({ children }: LayoutProps<"/staff">) {
  await connection();
  return children;
}
