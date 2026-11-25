"use client";

import * as React from "react";
import Link from "next/link";
import { CircleCheck, Gift, MailCheck, MessageCircle, PauseCircle, Share2, Sparkles } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import type { MyReferrals } from "@/lib/api/referrals";
import { useReferrals } from "@/lib/referrals/referral-provider";
import { useSignupFreeMinutes } from "@/lib/site-config/site-config-provider";
import { ErrorState } from "@/components/feedback/error-state";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Skeleton } from "@/components/ui/skeleton";
import { ReferralLinkField } from "@/components/referrals/referral-link-field";
import { friendsCompleted, friendsJoined, rewardSentence, rewardsLeft } from "@/components/referrals/referral-copy";

/**
 * /profile/invite (below the account tabs): how the invite program works, the link with copy/share,
 * the counters and the rules (FAQ). States: loading, error with retry,
 * program paused, before launch (minutes pending) and after launch.
 */
export function InvitePage() {
  const { text } = useI18n();
  const { data, error, reload } = useReferrals();

  return (
    <div className="flex flex-col gap-6 md:gap-8" data-testid="invite-page">
      {data ? (
        <InviteContent data={data} />
      ) : error ? (
        <ErrorState error={error} onRetry={reload} title={text("Your invite link could not be loaded")} />
      ) : (
        <div aria-busy="true" className="flex flex-col gap-4">
          <p role="status" className="sr-only">
            {text("Loading your invite link…")}
          </p>
          <Skeleton className="h-40 w-full rounded-[12px]" />
          <Skeleton className="h-24 w-full rounded-[12px]" />
        </div>
      )}
    </div>
  );
}

function InviteContent({ data }: { data: MyReferrals }) {
  const { text, locale } = useI18n();
  const { program, stats } = data;
  return (
    <>
      <section
        className="calc-olive flex flex-col gap-4 rounded-[12px] border-gold p-[18px] text-white md:rounded-[4px] md:border md:p-6"
        data-tone="dark"
        style={{ "--calc-texture": 0.2 } as React.CSSProperties}
        aria-labelledby="invite-hero-title"
      >
        <div className="flex items-start gap-3">
          <Gift aria-hidden className="mt-1 size-6 shrink-0 text-[#e9cf86]" strokeWidth={1.6} />
          <div className="flex flex-col gap-2">
            <h2 id="invite-hero-title" className="font-display text-xl md:text-2xl">
              {text("Invite friends, get free minutes")}
            </h2>
            <p className="text-[15px] leading-[1.5]">{rewardSentence(program, locale, text)}</p>
          </div>
        </div>
        {program.enabled ? <ReferralLinkField tone="dark" labelVisible /> : null}
      </section>

      <ProgramNotice program={program} />

      <section aria-labelledby="invite-stats-title" className="flex flex-col gap-3">
        <h2 id="invite-stats-title" className="text-xl font-semibold text-[#17150f]">
          {text("Your invites")}
        </h2>
        <dl className="grid grid-cols-2 gap-3 md:grid-cols-4" data-testid="referral-stats">
          <Stat label={text("Friends joined")} value={stats.joined} detail={friendsJoined(stats.joined, locale, text)} testId="stat-joined" />
          <Stat label={text("Completed a reading")} value={stats.qualified} detail={friendsCompleted(stats.qualified, locale, text)} testId="stat-qualified" />
          {program.launched ? (
            <Stat label={text("Minutes added")} value={stats.granted_minutes} testId="stat-granted" />
          ) : (
            <Stat label={text("Minutes pending until launch")} value={stats.pending_minutes} testId="stat-pending" />
          )}
          <Stat label={text("Rewards left")} display={rewardsLeft(stats, program, locale, text)} testId="stat-remaining" />
        </dl>
        {program.launched && stats.pending_minutes > 0 ? (
          <p className="text-[13px] text-[var(--muted-ink)]">
            {stats.pending_minutes === 1
              ? text("1 more minute is on its way to your wallet.")
              : text("{value0} more minutes are on their way to your wallet.", { value0: new Intl.NumberFormat(locale).format(stats.pending_minutes) })}
          </p>
        ) : null}
        {stats.remaining_rewards === 0 ? (
          <p className="text-[13px] text-[var(--muted-ink)]" data-testid="referral-cap-reached">
            {text("You reached the maximum number of rewarded friends. Thank you for spreading the word!")}
          </p>
        ) : null}
      </section>

      <Steps minutes={program.reward_minutes} />
      <Rules data={data} />
    </>
  );
}

function Stat({ label, value, display, detail, testId }: { label: string; value?: number; display?: string; detail?: string; testId: string }) {
  const { locale } = useI18n();
  return (
    <div className="flex flex-col gap-1 rounded-[12px] border border-[rgb(170_142_98/0.35)] bg-[rgb(251_246_233/0.35)] p-4 md:rounded-[4px]" data-testid={testId}>
      <dt className="text-[11px] font-medium tracking-[0.8px] text-[#74571a] uppercase">{label}</dt>
      <dd className="font-display text-2xl text-[#17150f]">
        {display ?? new Intl.NumberFormat(locale).format(value ?? 0)}
        {detail ? <span className="sr-only">{` (${detail})`}</span> : null}
      </dd>
    </div>
  );
}

function ProgramNotice({ program }: { program: MyReferrals["program"] }) {
  const { text, locale } = useI18n();
  const notice: { icon: typeof Sparkles; title: string; body: string } = !program.enabled
    ? {
        icon: PauseCircle,
        title: text("Invitations are paused"),
        body: text("New sign-ups with your link don't earn rewards right now. Minutes you already earned are kept."),
      }
    : program.launched
      ? {
          icon: Sparkles,
          title: text("Rewards are live"),
          body: program.launched_at
            ? text("Since {value0}, minutes are added to your wallet as soon as a friend completes a reading.", {
                value0: new Intl.DateTimeFormat(locale, { dateStyle: "long" }).format(new Date(program.launched_at)),
              })
            : text("Minutes are added to your wallet as soon as a friend completes a reading."),
        }
      : {
          icon: Sparkles,
          title: text("Staryield is in beta"),
          body: text("The minutes you earn now are kept for you and added to your wallet when we launch. We'll let you know."),
        };
  const Icon = notice.icon;
  return (
    <section
      className="flex items-start gap-3 rounded-[12px] border border-dashed border-[rgb(170_142_98/0.6)] p-4 md:rounded-[4px]"
      data-testid="referral-program-notice"
      data-state={!program.enabled ? "paused" : program.launched ? "launched" : "beta"}
    >
      <Icon aria-hidden className="mt-0.5 size-5 shrink-0 text-[#8a6a1f]" strokeWidth={1.6} />
      <div className="flex flex-col gap-0.5">
        <h2 className="text-base font-semibold text-[#17150f]">{notice.title}</h2>
        <p className="text-[13px] leading-[1.5] text-[var(--muted-ink)] md:text-sm">{notice.body}</p>
      </div>
    </section>
  );
}

function Steps({ minutes }: { minutes: number }) {
  const { text, locale } = useI18n();
  const steps = [
    { icon: Share2, title: text("Share your link"), body: text("Send it to friends who are curious about psychic readings, astrology or tarot.") },
    { icon: MailCheck, title: text("Your friend signs up"), body: text("They create a free account with your link and verify their e-mail address.") },
    {
      icon: MessageCircle,
      title: text("They complete a reading"),
      body:
        minutes === 1
          ? text("Once their first live reading ends, you earn 1 free minute.")
          : text("Once their first live reading ends, you earn {value0} free minutes.", { value0: new Intl.NumberFormat(locale).format(minutes) }),
    },
  ];
  return (
    <section aria-labelledby="invite-steps-title" className="flex flex-col gap-3">
      <h2 id="invite-steps-title" className="text-xl font-semibold text-[#17150f]">
        {text("How it works")}
      </h2>
      <ol className="grid gap-3 md:grid-cols-3">
        {steps.map((step, i) => (
          <li key={step.title} className="flex gap-3 rounded-[12px] bg-[rgb(251_246_233/0.35)] p-4 md:flex-col md:rounded-[4px]">
            <span className="flex size-9 shrink-0 items-center justify-center rounded-full border border-gold text-[#8a6a1f]" aria-hidden>
              <step.icon className="size-4" strokeWidth={1.6} />
            </span>
            <div className="flex flex-col gap-1">
              <h3 className="text-[15px] font-semibold text-[#17150f]">
                <span className="sr-only">{text("Step {value0}:", { value0: i + 1 })} </span>
                {step.title}
              </h3>
              <p className="text-[13px] leading-[1.5] text-[var(--muted-ink)]">{step.body}</p>
            </div>
          </li>
        ))}
      </ol>
    </section>
  );
}

function Rules({ data }: { data: MyReferrals }) {
  const { text, locale } = useI18n();
  const offer = useSignupFreeMinutes();
  const n = (v: number) => new Intl.NumberFormat(locale).format(v);
  const items: { q: string; a: string }[] = [
    {
      q: text("Who counts as an invited friend?"),
      a: text(
        "Someone who creates a new Staryield account with your link, verifies their e-mail address and completes a live reading of at least one minute, paid or free. Existing accounts, your own other accounts and friends who sign up without your link don't count.",
      ),
    },
    {
      q: text("When do I get my free minutes?"),
      a: text(
        "Staryield is in beta. Minutes you earn before we launch are shown as pending and added to your wallet at launch. After the launch, they are added as soon as your friend completes a reading.",
      ),
    },
    {
      q: text("Is there a limit?"),
      a:
        data.program.max_rewards === 1
          ? text("Each friend can be rewarded once, and you can be rewarded for 1 friend.")
          : text("Each friend can be rewarded once, and you can be rewarded for up to {value0} friends.", { value0: n(data.program.max_rewards) }),
    },
    {
      q: text("What does my friend get?"),
      a:
        offer > 0
          ? text("The same welcome offer as every new client: their first {value0} minutes of live chat are free. There is no extra reward for being invited.", {
              value0: n(offer),
            })
          : text("The same welcome offer as every new client. There is no extra reward for being invited."),
    },
    {
      q: text("Will I know who joined?"),
      a: text("No. To protect your friends' privacy, we only show how many friends joined and completed a reading, never who."),
    },
    {
      q: text("Can free minutes be exchanged for money?"),
      a: text(
        "No. Free minutes can only be used for live chat readings on Staryield. They have no cash value and cannot be transferred, exchanged or refunded.",
      ),
    },
    {
      q: text("Why might an invite not count?"),
      a: text(
        "Sign-ups from the same person, device or network and unusual patterns are reviewed and may not be rewarded. Accounts that break our Terms of Use are excluded. We may change or end the program; minutes already added to your wallet stay there.",
      ),
    },
  ];
  return (
    <section aria-labelledby="invite-rules-title" className="flex flex-col gap-2">
      <h2 id="invite-rules-title" className="text-xl font-semibold text-[#17150f]">
        {text("Questions and rules")}
      </h2>
      <Accordion type="multiple" className="flex flex-col" data-testid="referral-faq">
        {items.map((item, i) => (
          <AccordionItem key={i} value={`invite-faq-${i}`}>
            <AccordionTrigger>{item.q}</AccordionTrigger>
            <AccordionContent className="pb-4">
              <p className="text-sm leading-[1.6] text-[var(--muted-ink)]">{item.a}</p>
            </AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>
      <p className="mt-1 flex items-center gap-2 text-[13px] text-[var(--muted-ink)]">
        <CircleCheck aria-hidden className="size-4 text-[#8a6a1f]" strokeWidth={1.6} />
        <span>
          {text("Invites are subject to our")}{" "}
          <Link href={`/${locale}/terms`} className="underline underline-offset-2">
            {text("Terms of Use")}
          </Link>
        </span>
      </p>
    </section>
  );
}
