import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./schema";

// Browser client for the Go API through the /api/* rewrite (D-11). The
// session cookie is sent automatically; the CSRF token is added to every
// state-changing request (D-44).

let csrfToken = "";

export function setCsrfToken(token: string) {
  csrfToken = token;
}

const csrf: Middleware = {
  onRequest({ request }) {
    if (!["GET", "HEAD", "OPTIONS"].includes(request.method) && csrfToken) {
      request.headers.set("X-CSRF-Token", csrfToken);
    }
    return request;
  },
};

export const api = createClient<paths>({ baseUrl: "", credentials: "same-origin" });
api.use(csrf);

export type ApiError = {
  error: {
    code: string;
    message: string;
    request_id?: string;
    fields?: { field: string; message: string }[];
  };
};

export function isApiError(v: unknown): v is ApiError {
  return (
    typeof v === "object" && v !== null && "error" in v && typeof (v as ApiError).error?.message === "string"
  );
}

/** A message for any failure, including network errors where there is no body. */
export function errorMessage(v: unknown): string {
  return isApiError(v)
    ? v.error.message
    : "We could not reach the server. Check your connection and try again.";
}
