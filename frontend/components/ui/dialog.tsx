"use client";

import { useId, useRef, type ReactNode } from "react";
import { X } from "@/components/icons";
import { Button, type ButtonVariant } from "./button";

type DialogProps = {
  title: string;
  description?: string;
  triggerLabel: string;
  triggerVariant?: ButtonVariant;
  children: ReactNode;
  /**
   * Footer buttons. They sit in a <form method="dialog">, so any submit
   * button closes the dialog without JavaScript callbacks.
   */
  actions?: ReactNode;
};

/**
 * Modal dialog on the native <dialog> element: the browser traps focus, makes
 * the page behind inert, closes on Escape and returns focus to the trigger.
 */
export function Dialog({
  title,
  description,
  triggerLabel,
  triggerVariant = "secondary",
  children,
  actions,
}: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const descId = useId();
  const close = () => ref.current?.close();

  return (
    <>
      <Button variant={triggerVariant} onClick={() => ref.current?.showModal()}>
        {triggerLabel}
      </Button>
      <dialog
        ref={ref}
        aria-labelledby={titleId}
        aria-describedby={description ? descId : undefined}
        // Clicking the backdrop (the dialog element itself) closes it.
        onClick={(e) => {
          if (e.target === e.currentTarget) close();
        }}
        className="m-auto w-[calc(100%-2rem)] max-w-lg rounded-card border border-stone bg-white p-0 text-ink shadow-xl backdrop:bg-navy-900/60"
      >
        <div className="flex flex-col gap-4 p-5">
          <div className="flex items-start justify-between gap-4">
            <h2 id={titleId} className="text-2xl font-semibold">
              {title}
            </h2>
            <button
              type="button"
              onClick={close}
              className="-m-2 inline-flex min-h-touch min-w-touch items-center justify-center rounded-control text-ink-muted hover:bg-stone/50"
            >
              <X aria-hidden className="size-6" />
              <span className="sr-only">Close</span>
            </button>
          </div>
          {description ? (
            <p id={descId} className="text-ink-muted">
              {description}
            </p>
          ) : null}
          <div>{children}</div>
          {actions ? (
            <form method="dialog" className="flex flex-wrap justify-end gap-3">
              {actions}
            </form>
          ) : null}
        </div>
      </dialog>
    </>
  );
}
