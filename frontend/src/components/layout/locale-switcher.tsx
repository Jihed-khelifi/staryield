"use client";

import { usePathname, useRouter } from "next/navigation";
import { FiCheck, FiChevronDown, FiGlobe } from "react-icons/fi";

import { localeLabels, locales, type Locale } from "@/i18n/config";
import { useI18n } from "@/i18n/i18n-provider";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export function LocaleSwitcher({ className }: { className?: string }) {
  const { locale, t } = useI18n();
  const pathname = usePathname();
  const router = useRouter();

  function switchTo(next: Locale) {
    if (next === locale) return;

    // Remember the choice so the proxy keeps honouring it on later visits.
    // Writing the locale cookie is an intentional browser side effect in this event handler.
    // eslint-disable-next-line react-hooks/immutability
    document.cookie = `STARYIELD_LOCALE=${next}; path=/; max-age=31536000; samesite=lax`;

    const segments = pathname.split("/");
    segments[1] = next;
    router.replace(segments.join("/") || `/${next}`);
    router.refresh();
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          aria-label={t("nav.changeLanguage")}
          className={cn(
            "gap-1.5 font-serif text-ink hover:bg-gold/15",
            className,
          )}
        >
          <FiGlobe aria-hidden />
          <span>{localeLabels[locale].short}</span>
          <FiChevronDown aria-hidden className="opacity-60" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-40 font-serif">
        {locales.map((option) => (
          <DropdownMenuItem
            key={option}
            onSelect={() => switchTo(option)}
            className="justify-between"
          >
            <span>{localeLabels[option].name}</span>
            {option === locale ? (
              <FiCheck aria-hidden className="text-amber" />
            ) : null}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
