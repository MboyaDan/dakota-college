"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { CircleCheck, CircleX, Info, X } from "@/components/icons";
import { cn } from "@/lib/cn";

type ToastTone = "info" | "success" | "danger";
type Toast = { id: number; tone: ToastTone; title: string; description?: string };
type ShowToast = (toast: Omit<Toast, "id">) => void;

const ToastContext = createContext<ShowToast | null>(null);

/** Errors stay until dismissed; other toasts leave after this long. */
const AUTO_DISMISS_MS = 6000;

const toneStyle: Record<ToastTone, { icon: typeof Info; className: string }> = {
  info: { icon: Info, className: "border-royal" },
  success: { icon: CircleCheck, className: "border-success" },
  danger: { icon: CircleX, className: "border-danger" },
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextId = useRef(1);

  const dismiss = useCallback((id: number) => setToasts((t) => t.filter((x) => x.id !== id)), []);
  const show = useCallback<ShowToast>((toast) => {
    const id = nextId.current++;
    setToasts((t) => [...t, { ...toast, id }]);
  }, []);

  const polite = toasts.filter((t) => t.tone !== "danger");
  const urgent = toasts.filter((t) => t.tone === "danger");

  return (
    <ToastContext.Provider value={show}>
      {children}
      {/* Live regions are always mounted so additions are announced. */}
      <div className="pointer-events-none fixed inset-x-0 bottom-0 z-50 flex flex-col items-center gap-2 p-4 sm:items-end">
        <div role="status" aria-live="polite" className="contents">
          {polite.map((t) => (
            <ToastCard key={t.id} toast={t} onDismiss={dismiss} />
          ))}
        </div>
        <div role="alert" aria-live="assertive" className="contents">
          {urgent.map((t) => (
            <ToastCard key={t.id} toast={t} onDismiss={dismiss} />
          ))}
        </div>
      </div>
    </ToastContext.Provider>
  );
}

function ToastCard({ toast, onDismiss }: { toast: Toast; onDismiss: (id: number) => void }) {
  const { icon: Icon, className } = toneStyle[toast.tone];

  useEffect(() => {
    if (toast.tone === "danger") return;
    const timer = setTimeout(() => onDismiss(toast.id), AUTO_DISMISS_MS);
    return () => clearTimeout(timer);
  }, [toast, onDismiss]);

  return (
    <div
      className={cn(
        "pointer-events-auto flex w-full max-w-sm gap-3 rounded-card border-l-4 bg-white p-4 text-ink shadow-lg",
        className,
      )}
    >
      <Icon aria-hidden className="mt-0.5 size-5 shrink-0" />
      <div className="flex flex-1 flex-col gap-0.5">
        <p className="font-semibold">{toast.title}</p>
        {toast.description ? <p className="text-sm text-ink-muted">{toast.description}</p> : null}
      </div>
      <button
        type="button"
        onClick={() => onDismiss(toast.id)}
        className="-m-2 inline-flex min-h-touch min-w-touch items-center justify-center rounded-control text-ink-muted hover:bg-stone/50"
      >
        <X aria-hidden className="size-5" />
        <span className="sr-only">Dismiss</span>
      </button>
    </div>
  );
}

export function useToast() {
  const show = useContext(ToastContext);
  if (!show) throw new Error("useToast must be used inside <ToastProvider>");
  return useMemo(() => ({ show }), [show]);
}
