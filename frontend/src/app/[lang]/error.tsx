"use client";

import { useEffect } from "react";
import Link from "next/link";

import { useI18n } from "@/i18n/i18n-provider";
import { logger } from "@/lib/logger";
import { Button } from "@/components/ui/button";

/** Route-level error boundary: logs the failure and offers recovery. */
export default function RouteError({ error, retry }: { error: Error & { digest?: string }; retry: () => void }) {
  const { text, locale } = useI18n();
  useEffect(() => {
    logger.error(`render failed: ${error.message}`, error, { code: "render_failed", digest: error.digest ?? "" });
  }, [error]);
  return (
    <main className="error-page paper-page" role="alert">
      <h1 className="font-display text-3xl">{text("Something went wrong")}</h1>
      <p className="mt-4 max-w-md text-center font-light">
        {text("This page could not be displayed. You can try again, or return to the home page.")}
      </p>
      {error.digest ? <p className="mt-2 text-xs text-sage">{text("Reference: {value0}", { value0: error.digest })}</p> : null}
      <div className="mt-8 flex gap-4">
        <Button className="gold-button" onClick={() => retry()}>
          {text("Try again")}
        </Button>
        <Button asChild variant="outline">
          <Link href={`/${locale}`}>{text("Home")}</Link>
        </Button>
      </div>
    </main>
  );
}
