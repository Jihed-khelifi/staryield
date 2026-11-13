import type { Locale } from "@/i18n/config";
import type { LegalDocumentContent, LegalDocumentId, LegalPolicyLayout } from "@/content/legal/types";
import { Button } from "@/components/ui/button";
import { Block } from "@/components/content/legal/legal-blocks";
import { resolveHref } from "@/components/content/legal/inline-text";
import { cn } from "@/lib/utils";

/** "11.11.2026": the numeric date the policy header uses (Figma 1360:2814). */
const policyDate = (iso: string) => iso.split("-").reverse().join(".");

const focusRing = "focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#c29a3d]";
const textLink = cn("inline-flex min-h-11 items-center font-serif text-base text-[#3d332a] underline decoration-[#c29a3d] underline-offset-2 hover:text-ink", focusRing);

function ContentsList({ doc }: { doc: LegalDocumentContent }) {
  return (
    <ol className="flex flex-col">
      {doc.sections.map((section) => (
        <li key={section.id}>
          <a href={`#${section.id}`} className={cn("flex min-h-11 items-center font-serif text-base leading-normal text-[#3d332a] underline-offset-2 hover:text-ink hover:underline", focusRing)}>
            {`${section.n}. ${section.title}`}
          </a>
        </li>
      ))}
    </ol>
  );
}

function HeaderActions({ actions, locale }: { actions: LegalPolicyLayout["actions"]; locale: Locale }) {
  return (
    <div className="flex w-full flex-col gap-4 sm:flex-row">
      {actions.map((action, index) => {
        const href = resolveHref(action.href, locale);
        if (!href) return null;
        return (
          <Button
            key={action.href}
            asChild
            variant={index === 0 ? "cta" : "outline"}
            size="xl"
            className={cn("h-12 rounded-md px-6 text-base sm:w-52", index > 0 && "border-gold bg-transparent font-semibold text-ink shadow-glow hover:bg-cream/50")}
          >
            <a href={href}>{action.label}</a>
          </Button>
        );
      })}
    </div>
  );
}

/**
 * Policy reading layout (Figma 1360:2766 desktop / 1360:2771 mobile):
 * left-aligned header with actions, a sticky "On this page" list beside a
 * reading card (collapsible list above it on phones and tablets), and the
 * related links at the end of the card. Server-rendered, no client
 * JavaScript (the phone list is a native <details>).
 */
export function PolicyDocument({ id, doc, locale }: { id: LegalDocumentId; doc: LegalDocumentContent; locale: Locale }) {
  const { meta, labels } = doc;
  const policy = doc.policy as LegalPolicyLayout;
  return (
    <div className="w-full bg-cream print:bg-white" data-legal-document={id} data-legal-layout="policy">
      <div className="w-full px-4 pt-8 pb-4 sm:px-6 md:pt-12 lg:px-12">
        <div className="mx-auto flex w-full max-w-[1184px] flex-col gap-6">
          <nav aria-label={policy.breadcrumb.current} className="lg:hidden">
            <ol className="flex items-center gap-3 font-serif text-base text-[#3d332a]">
              <li>
                <a href={`/${locale}`} className={cn("inline-flex min-h-11 items-center underline underline-offset-2", focusRing)}>
                  {policy.breadcrumb.home}
                </a>
              </li>
              <li aria-hidden>/</li>
              <li aria-current="page">{policy.breadcrumb.current}</li>
            </ol>
          </nav>
          <header className="flex flex-col gap-4">
            <p className="font-serif text-base tracking-[1px] text-[#3d332a] uppercase">
              <span aria-hidden>✦ </span>
              {meta.badge}
            </p>
            <h1 id="legal-title" className="font-display text-[clamp(1.875rem,5vw,3rem)] leading-tight text-ink">
              {meta.title}
            </h1>
          </header>
          <p className="flex flex-col gap-2 font-serif text-base leading-relaxed text-[#3d332a] sm:flex-row sm:gap-6">
            <span>
              {labels.effective} <time dateTime={meta.effective}>{policyDate(meta.effective)}</time>
            </span>
            <span>
              {labels.lastUpdated} <time dateTime={meta.lastUpdated}>{policyDate(meta.lastUpdated)}</time>
            </span>
          </p>
          <HeaderActions actions={policy.actions} locale={locale} />
        </div>
      </div>

      <div className="w-full px-4 pt-4 pb-16 sm:px-6 lg:px-12">
        <div className="mx-auto flex w-full max-w-[1184px] flex-col gap-6 lg:flex-row lg:items-start lg:gap-8">
          <details className="rounded-md border border-warm-gray bg-cream-light lg:hidden">
            <summary className={cn("flex min-h-11 cursor-pointer items-center px-4 font-serif text-base font-semibold text-[#3d332a] uppercase", focusRing)}>{labels.toc}</summary>
            <nav id="toc-mobile" aria-label={labels.toc} className="px-4 pb-3">
              <ContentsList doc={doc} />
            </nav>
          </details>

          <aside className="hidden w-[232px] shrink-0 lg:sticky lg:top-24 lg:block lg:max-h-[calc(100dvh-8rem)] lg:overflow-y-auto">
            <nav id="toc" aria-labelledby="legal-toc-title" className="py-6">
              <h2 id="legal-toc-title" className="mb-3 font-serif text-base leading-snug font-semibold text-[#3d332a] uppercase">
                {labels.toc}
              </h2>
              <ContentsList doc={doc} />
            </nav>
          </aside>

          <article
            aria-labelledby="legal-title"
            className="paper-overlay min-w-0 flex-1 rounded-md border border-warm-gray bg-cream px-5 py-10 font-serif text-base leading-[1.75] text-[#2d241e] sm:px-10 md:px-20 md:py-12 md:text-[18px] lg:max-w-[920px] print:border-0 print:px-0"
          >
            <div className="relative z-10 flex flex-col gap-8 wrap-anywhere [&_h4]:mt-2 [&_h4]:mb-0 [&_h4]:text-[20px] [&_h4]:leading-snug [&_p]:my-0 [&_ul]:my-0">
              <div className="flex flex-col gap-4">
                {doc.intro.map((block, index) => (
                  <Block key={index} block={block} locale={locale} />
                ))}
              </div>
              {doc.sections.map((section) => (
                <section key={section.id} aria-labelledby={section.id} className="flex flex-col gap-4 border-t border-warm-gray pt-6">
                  <h2 id={section.id} className="scroll-mt-28 font-serif text-2xl leading-snug font-semibold text-[#1a1612]">
                    {`${section.n}. ${section.title}`}
                  </h2>
                  {section.blocks.map((block, index) => (
                    <Block key={index} block={block} locale={locale} />
                  ))}
                </section>
              ))}
              <nav aria-labelledby="legal-related-title" className="flex flex-col gap-3 border-t border-warm-gray pt-6">
                <p id="legal-related-title" className="text-lg leading-snug font-semibold text-[#1a1612]">
                  {policy.related.title}
                </p>
                <ul className="flex flex-col gap-1 md:flex-row md:gap-6">
                  {policy.related.links.map((link) => {
                    const href = resolveHref(link.href, locale);
                    return href ? (
                      <li key={link.href + link.label}>
                        <a href={href} className={textLink}>
                          {link.label}
                        </a>
                      </li>
                    ) : null;
                  })}
                </ul>
              </nav>
            </div>
          </article>
        </div>
      </div>
    </div>
  );
}
