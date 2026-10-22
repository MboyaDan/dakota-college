import type { Metadata } from "next";

// Staff portal: never indexed (brief 12.1).
export const metadata: Metadata = {
  title: {
    template: "%s | Staff portal | Dakota Training College",
    default: "Staff portal | Dakota Training College",
  },
  robots: { index: false, follow: false },
};

export default function StaffLayout({ children }: LayoutProps<"/staff">) {
  return children;
}
