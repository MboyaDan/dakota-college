import { connection } from "next/server";

// The application form pages hold the applicant's personal data. They render
// per request, with no prerendered shell, so Next.js can put this response's
// script nonce on every script (D-191). The /apply landing page stays cached.
export const instant = false;

export default async function ApplicationFormLayout({ children }: { children: React.ReactNode }) {
  await connection();
  return children;
}
