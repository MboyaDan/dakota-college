import { NextResponse, type NextRequest } from "next/server";
import {
  contentSecurityPolicy,
  forwardClientIP,
  isStrictPath,
  resolveContentImageOrigin,
} from "@/lib/security-headers";

const imageOrigin = resolveContentImageOrigin(process.env.CONTENT_IMAGE_ORIGIN);
const dev = process.env.NODE_ENV === "development";

/**
 * API calls get the visitor's address added (R-23). Pages get their
 * Content-Security-Policy: a fresh nonce on signed-in pages, which Next.js
 * reads from the request header and puts on its own scripts, and a fixed
 * policy on cached public pages (D-191).
 */
export function proxy(request: NextRequest) {
  const headers = new Headers(request.headers);
  if (request.nextUrl.pathname.startsWith("/api/")) {
    forwardClientIP(headers, process.env.CLIENT_IP_HEADER, process.env.CLIENT_IP_SECRET);
    return NextResponse.next({ request: { headers } });
  }

  let nonce: string | undefined;
  if (isStrictPath(request.nextUrl.pathname)) {
    nonce = btoa(String.fromCharCode(...crypto.getRandomValues(new Uint8Array(18))));
  }
  const policy = contentSecurityPolicy({ nonce, imageOrigin, dev });
  if (nonce) {
    headers.set("content-security-policy", policy);
  }
  const response = NextResponse.next({ request: { headers } });
  response.headers.set("content-security-policy", policy);
  return response;
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
