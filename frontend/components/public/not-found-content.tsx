import Link from "next/link";
import { getSite } from "@/lib/public/site";
import { FinderForm } from "./finder-form";

/** Body of the 404 page: say what happened and offer the likely next steps. */
export async function NotFoundContent() {
  const site = await getSite();
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-16 lg:px-6">
      <p className="font-semibold text-royal">Page not found</p>
      <h1 className="text-4xl font-semibold text-navy-900">We could not find that page</h1>
      <p className="text-lg">
        The address may be mistyped, or the page may have moved or been taken down. Try searching for a
        course, or use one of these links.
      </p>
      <div className="rounded-card border border-stone bg-white p-5">
        <FinderForm schools={site.schools} compact idPrefix="notfound-finder" />
      </div>
      <ul className="flex flex-wrap gap-x-6 gap-y-1 font-semibold">
        {[
          ["/", "Home"],
          ["/programmes", "Course finder"],
          ["/admissions", "How to apply"],
          ["/contact", "Contact us"],
        ].map(([href, label]) => (
          <li key={href}>
            <Link href={href} className="inline-flex min-h-touch items-center text-royal underline">
              {label}
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
