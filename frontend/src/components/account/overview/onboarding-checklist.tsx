"use client";

import * as React from "react";
import Link from "next/link";
import { CalendarDays, Check, MailCheck, MessageCircle, UserPlus, type LucideIcon } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { ONBOARDING_UPDATED, onboardingApi, type Onboarding, type OnboardingStepKey } from "@/lib/api/onboarding";
import { useRealtimeEvents, useSession } from "@/lib/auth/session-provider";
import { usePsychicsHref } from "@/lib/psychics-link";
import { useAsync } from "@/lib/use-async";
import { cn } from "@/lib/utils";
import { ErrorState } from "@/components/feedback/error-state";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Account facts that complete a step elsewhere in the app. The API recomputes
 * the checklist, so any of them only triggers a refetch: the e-mail address was
 * confirmed (any tab), a reading ended or was paid for, a lifecycle message
 * arrived. `onboarding.updated` carries the new checklist itself (hidden or
 * shown again in another tab or device).
 */
export const ONBOARDING_REFRESH_EVENTS = ["account.email_verified", "wallet.updated", "session.ended", "notification.created"] as const;

type StepCopy = { icon: LucideIcon; title: string; description: string; cta: string; doneLabel: string };

const STEPS: Record<OnboardingStepKey, StepCopy> = {
  verify_email: {
    icon: MailCheck,
    title: "Confirm your email address",
    description: "Keep your account secure and make sure our emails reach you.",
    cta: "Confirm email",
    doneLabel: "Email confirmed",
  },
  birth_details: {
    icon: CalendarDays,
    title: "Add your birth details",
    description: "Your date and place of birth personalise your horoscope and chart.",
    cta: "Add details",
    doneLabel: "Birth details added",
  },
  first_reading: {
    icon: MessageCircle,
    title: "Have your first reading",
    description: "Chat live with the advisor of your choice.",
    cta: "Choose an advisor",
    doneLabel: "First reading done",
  },
  invite_friend: {
    icon: UserPlus,
    title: "Invite a friend",
    description: "Share Staryield with someone who would enjoy it.",
    cta: "Get your invite link",
    doneLabel: "Friend invited",
  },
};

/** Where each step's call to action leads (the e-mail address and the birth details are both in "Account Information"). */
function stepHref(key: OnboardingStepKey, psychicsHref: string, locale: string): string {
  switch (key) {
    case "verify_email":
    case "birth_details":
      return `/${locale}/profile/settings#account`;
    case "first_reading":
      return psychicsHref;
    case "invite_friend":
      return `/${locale}/profile/invite`;
  }
}

function isOnboarding(value: unknown): value is Onboarding {
  const v = value as Onboarding | null;
  return Boolean(v && Array.isArray(v.steps) && typeof v.completed === "number" && typeof v.total === "number" && typeof v.dismissed === "boolean");
}

/**
 * The client's "Get started" checklist on the Account Overview: the next steps
 * that make the most of Staryield (confirm the e-mail address, add birth
 * details, a first reading, invite a friend), with progress. Each open step
 * links to where it is done; the card can be hidden (with an undo) and hides
 * itself once every step is done. Advisors and admins never see it.
 */
export function OnboardingChecklist() {
  const { text, locale } = useI18n();
  const { user } = useSession();
  const toast = useToast();
  const psychicsHref = usePsychicsHref();
  const isClient = user?.role === "client";
  // The birth details live on the session user: saving them in settings
  // refetches the checklist here at once.
  const state = useAsync((signal) => onboardingApi.get(signal), [user?.id, user?.profile.birth_date, user?.profile.birth_place_id], isClient);
  const { data, error, loading, reload, setData } = state;
  const [busy, setBusy] = React.useState(false);

  // Several events arrive together when a reading ends: refetch once.
  const timer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  React.useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);
  useRealtimeEvents<unknown>([ONBOARDING_UPDATED, ...ONBOARDING_REFRESH_EVENTS], (type, payload) => {
    if (!isClient) return;
    if (type === ONBOARDING_UPDATED && isOnboarding(payload)) {
      setData(payload);
      return;
    }
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(reload, 400);
  });

  async function restore() {
    try {
      setData(await onboardingApi.restore());
      toast.show({ tone: "success", message: text("Your checklist is back.") });
    } catch (err) {
      toast.showError(err);
    }
  }

  async function dismiss() {
    setBusy(true);
    try {
      const next = await onboardingApi.dismiss();
      track("onboarding_dismissed");
      setData(next);
      toast.show({ tone: "info", message: text("Checklist hidden."), actionLabel: text("Undo"), onAction: () => void restore(), durationMs: 10_000 });
    } catch (err) {
      toast.showError(err);
    } finally {
      setBusy(false);
    }
  }

  if (!isClient) return null;
  if (!data) {
    if (error) {
      return (
        <section aria-labelledby="onboarding-title" data-testid="onboarding-checklist" data-state="error">
          <h2 id="onboarding-title" className="sr-only">
            {text("Get started")}
          </h2>
          <ErrorState error={error} onRetry={reload} compact title={text("Your checklist could not be loaded")} />
        </section>
      );
    }
    return loading ? (
      <div aria-busy="true" data-testid="onboarding-checklist" data-state="loading" className="rounded-[12px] border border-gold p-[18px] md:rounded-[4px] md:p-5">
        <p role="status" className="sr-only">
          {text("Loading your checklist…")}
        </p>
        <Skeleton className="h-6 w-40" />
        <Skeleton className="mt-3 h-1.5 w-full rounded-full" />
        <div className="mt-4 grid gap-2.5 md:grid-cols-2">
          <Skeleton className="h-[74px] w-full rounded-[12px] md:rounded-[4px]" />
          <Skeleton className="h-[74px] w-full rounded-[12px] md:rounded-[4px]" />
        </div>
      </div>
    ) : null;
  }
  if (data.dismissed || data.total === 0 || data.completed >= data.total) return null;

  const progress = text("{completed} of {total} done", { completed: data.completed, total: data.total });
  // The next open step stands out, so what to do now is obvious.
  const next = data.steps.find((step) => !step.done)?.key;
  return (
    <section
      aria-labelledby="onboarding-title"
      className="rounded-[12px] border border-gold bg-[rgb(251_246_233/0.35)] p-[18px] shadow-[0_10px_28px_rgb(23_21_15/0.06)] md:rounded-[4px] md:bg-warm-gray md:p-5 md:shadow-none"
      data-testid="onboarding-checklist"
      data-state="ready"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-[3px] md:gap-1">
          <p className="text-[10px] font-medium tracking-[1.5px] text-[#74571a] uppercase md:hidden">{text("Your first steps")}</p>
          <h2 id="onboarding-title" className="text-xl font-semibold text-[#17150f] md:text-black">
            {text("Get started")}
          </h2>
          <p className="text-[13px] text-[var(--muted-ink)] md:text-sm" data-testid="onboarding-progress-text">
            {progress}
          </p>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="shrink-0 text-[var(--muted-ink)]"
          onClick={() => void dismiss()}
          disabled={busy}
          aria-busy={busy || undefined}
          aria-label={text("Hide the checklist")}
          data-testid="onboarding-dismiss"
        >
          {text("Hide")}
        </Button>
      </div>
      <div
        role="progressbar"
        aria-label={text("Get started")}
        aria-valuemin={0}
        aria-valuemax={data.total}
        aria-valuenow={data.completed}
        aria-valuetext={progress}
        className="mt-3 h-1.5 w-full overflow-hidden rounded-full bg-[rgb(170_142_98/0.25)]"
        data-testid="onboarding-progress"
      >
        <div className="h-full rounded-full bg-[#8a6a1f] transition-[width] duration-500 motion-reduce:transition-none" style={{ width: `${(data.completed / data.total) * 100}%` }} />
      </div>
      <ol className="mt-4 grid gap-2.5 md:grid-cols-2 md:gap-3">
        {data.steps.map((step) => {
          const copy = STEPS[step.key];
          if (!copy) return null;
          const Icon = step.done ? Check : copy.icon;
          return (
            <li
              key={step.key}
              className={cn(
                "flex items-center gap-3 rounded-[12px] border border-gold p-[14px] md:gap-4 md:rounded-[4px] md:p-4",
                step.done ? "bg-transparent" : "bg-[#d6d0bc] md:bg-[rgb(251_246_233/0.6)]",
              )}
              data-step={step.key}
              data-done={step.done || undefined}
              data-testid="onboarding-step"
            >
              <span
                aria-hidden
                className={cn(
                  "flex size-[38px] shrink-0 items-center justify-center rounded-full border border-gold md:size-10",
                  step.done ? "bg-[#8a6a1f] text-[#fbf6e9]" : "bg-sage text-[#fbf6e9]",
                )}
              >
                <Icon className="size-5" strokeWidth={1.5} />
              </span>
              <div className="flex min-w-0 flex-1 flex-col gap-[3px] md:gap-0.5">
                <h3 className={cn("text-[15px] font-semibold md:text-base", step.done ? "text-[var(--muted-ink)] line-through decoration-1" : "text-[#17150f] md:text-black")}>
                  {text(copy.title)}
                </h3>
                <p className="text-[11px] text-[var(--muted-ink)] md:text-[13px]">{step.done ? text(copy.doneLabel) : text(copy.description)}</p>
              </div>
              {step.done ? (
                <span className="sr-only">{text("Completed")}</span>
              ) : (
                <Button asChild variant={step.key === next ? "cta" : "outline"} size="sm" className="shrink-0" data-next={step.key === next || undefined}>
                  <Link href={stepHref(step.key, psychicsHref, locale)} onClick={() => track("onboarding_step_clicked", { step: step.key })} data-testid={`onboarding-cta-${step.key}`}>
                    {text(copy.cta)}
                  </Link>
                </Button>
              )}
            </li>
          );
        })}
      </ol>
    </section>
  );
}
