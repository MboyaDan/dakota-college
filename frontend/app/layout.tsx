import type { Metadata, Viewport } from "next";
import { inter, newsreader, plexMono } from "@/lib/fonts";
import "./globals.css";

export const metadata: Metadata = {
  title: "Dakota Training College",
  // Scaffold only. Stage 6 replaces this with real SEO metadata.
  robots: { index: false, follow: false },
};

export const viewport: Viewport = {
  themeColor: "#0A1F4D",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${inter.variable} ${newsreader.variable} ${plexMono.variable} h-full antialiased`}
    >
      <body className="flex min-h-full flex-col">{children}</body>
    </html>
  );
}
