"use client";

import { useId, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { cn } from "@/lib/cn";

export type TabItem = { id: string; label: string; content: ReactNode };

type TabsProps = {
  items: TabItem[];
  /** Names the tab list for screen readers, e.g. "Programme levels". */
  label: string;
  defaultTabId?: string;
};

/**
 * WAI-ARIA tabs with automatic activation: arrow keys, Home and End move
 * between tabs; Tab moves into the panel.
 */
export function Tabs({ items, label, defaultTabId }: TabsProps) {
  const baseId = useId();
  const [active, setActive] = useState(defaultTabId ?? items[0]?.id);
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);

  function onKeyDown(e: KeyboardEvent<HTMLButtonElement>, index: number) {
    const last = items.length - 1;
    const next = {
      ArrowRight: index === last ? 0 : index + 1,
      ArrowLeft: index === 0 ? last : index - 1,
      Home: 0,
      End: last,
    }[e.key];
    if (next === undefined) return;
    e.preventDefault();
    setActive(items[next].id);
    tabRefs.current[next]?.focus();
  }

  return (
    <div className="flex flex-col gap-4">
      <div role="tablist" aria-label={label} className="flex gap-1 overflow-x-auto border-b border-stone">
        {items.map((item, i) => {
          const selected = item.id === active;
          return (
            <button
              key={item.id}
              ref={(el) => {
                tabRefs.current[i] = el;
              }}
              type="button"
              role="tab"
              id={`${baseId}-tab-${item.id}`}
              aria-selected={selected}
              aria-controls={`${baseId}-panel-${item.id}`}
              tabIndex={selected ? 0 : -1}
              onClick={() => setActive(item.id)}
              onKeyDown={(e) => onKeyDown(e, i)}
              className={cn(
                "-mb-px min-h-touch shrink-0 border-b-4 px-4 py-2 font-semibold whitespace-nowrap",
                selected
                  ? "border-gold text-navy-900"
                  : "border-transparent text-ink-muted hover:text-navy-900",
              )}
            >
              {item.label}
            </button>
          );
        })}
      </div>
      {items.map((item) => (
        <div
          key={item.id}
          role="tabpanel"
          id={`${baseId}-panel-${item.id}`}
          aria-labelledby={`${baseId}-tab-${item.id}`}
          hidden={item.id !== active}
          tabIndex={0}
        >
          {item.content}
        </div>
      ))}
    </div>
  );
}
