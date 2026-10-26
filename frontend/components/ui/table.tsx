import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type Column<Row> = {
  key: string;
  header: string;
  cell: (row: Row) => ReactNode;
  /** Right-aligns with tabular figures, for money and counts. */
  numeric?: boolean;
};

type TableProps<Row> = {
  caption: string;
  columns: Column<Row>[];
  rows: Row[];
  rowKey: (row: Row) => string;
  /** Shown in place of the table body when there are no rows. */
  empty?: ReactNode;
  /** Hide the caption visually (it is still read by screen readers). */
  captionHidden?: boolean;
};

export function Table<Row>({ caption, columns, rows, rowKey, empty, captionHidden }: TableProps<Row>) {
  return (
    // Focusable so keyboard users can scroll wide tables on small screens.
    // Relative, so screen-reader-only text in cells stays inside the scroll
    // box instead of widening the page.
    <div
      role="region"
      aria-label={caption}
      tabIndex={0}
      className="relative overflow-x-auto rounded-card border border-stone bg-white"
    >
      <table className="w-full border-collapse text-left">
        <caption
          className={cn("px-4 py-3 text-left font-semibold text-navy-900", captionHidden && "sr-only")}
        >
          {caption}
        </caption>
        <thead className="bg-ivory">
          <tr>
            {columns.map((c) => (
              <th
                key={c.key}
                scope="col"
                className={cn(
                  "border-b border-stone px-4 py-3 font-semibold whitespace-nowrap",
                  c.numeric && "text-right",
                )}
              >
                {c.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td colSpan={columns.length} className="px-4 py-6">
                {empty ?? "Nothing to show yet."}
              </td>
            </tr>
          ) : (
            rows.map((row) => (
              <tr key={rowKey(row)} className="border-b border-stone last:border-b-0">
                {columns.map((c) => (
                  <td
                    key={c.key}
                    className={cn("px-4 py-3 align-top", c.numeric && "tabular text-right whitespace-nowrap")}
                  >
                    {c.cell(row)}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
