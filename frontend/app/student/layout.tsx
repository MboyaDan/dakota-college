import type { Metadata } from "next";

// Student portal (brief 7.1): never indexed.
export const metadata: Metadata = {
  title: {
    template: "%s | Student portal | Dakota Training College",
    default: "Student portal | Dakota Training College",
  },
  robots: { index: false, follow: false },
};

export default function StudentLayout({ children }: LayoutProps<"/student">) {
  return children;
}
