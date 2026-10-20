"use client";

import * as React from "react";
import { X } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { errorMessage } from "@/lib/api/errors";
import { cn } from "@/lib/utils";

type Tone = "info" | "success" | "error";

type Toast = {
  id: number;
  tone: Tone;
  message: string;
  actionLabel?: string;
  onAction?: () => void;
};

type ToastInput = Omit<Toast, "id"> & { durationMs?: number };

type ToastContextValue = {
  show: (toast: ToastInput) => void;
  /** Show the localized message for an API error. */
  showError: (error: unknown, fallback?: string) => void;
};

const ToastContext = React.createContext<ToastContextValue | null>(null);

/** Accessible, polite notification stack (messages are already translated). */
export function ToastProvider({ children }: { children: React.ReactNode }) {
  const { text } = useI18n();
  const [toasts, setToasts] = React.useState<Toast[]>([]);
  const nextId = React.useRef(0);

  const dismiss = React.useCallback((id: number) => {
    setToasts((all) => all.filter((t) => t.id !== id));
  }, []);

  const show = React.useCallback(
    ({ durationMs = 6000, ...toast }: ToastInput) => {
      const id = ++nextId.current;
      setToasts((all) => [...all.filter((t) => t.message !== toast.message).slice(-3), { ...toast, id }]);
      if (durationMs > 0) window.setTimeout(() => dismiss(id), durationMs);
    },
    [dismiss],
  );

  const showError = React.useCallback(
    (error: unknown, fallback?: string) => show({ tone: "error", message: text(fallback ?? errorMessage(error)) }),
    [show, text],
  );

  const value = React.useMemo(() => ({ show, showError }), [show, showError]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div aria-live="polite" role="status" className="toast-region">
        {toasts.map((t) => (
          <div key={t.id} className={cn("toast", `toast-${t.tone}`)} role={t.tone === "error" ? "alert" : undefined}>
            <p>{t.message}</p>
            {t.actionLabel && t.onAction ? (
              <button
                type="button"
                className="toast-action"
                onClick={() => {
                  t.onAction?.();
                  dismiss(t.id);
                }}
              >
                {t.actionLabel}
              </button>
            ) : null}
            <button type="button" aria-label={text("Dismiss notification")} className="toast-close" onClick={() => dismiss(t.id)}>
              <X size={14} aria-hidden />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = React.useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used inside <ToastProvider>");
  return ctx;
}
