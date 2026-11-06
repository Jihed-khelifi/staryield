"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { Skeleton, SkeletonText } from "@/components/ui/skeleton";

/**
 * Suspense fallback for slow route segments: a header-shaped bar (so the
 * chrome doesn't jump) and a content skeleton, announced once as
 * "Loading…".
 */
export function RouteLoading({ variant = "page" }: { variant?: "page" | "chat" }) {
  const { text } = useI18n();
  return (
    <div className="dashboard-page mx-auto flex min-h-dvh w-full max-w-[1440px] flex-col gap-4" aria-busy="true">
      <p role="status" className="sr-only">
        {text("Loading…")}
      </p>
      <div aria-hidden className="site-header">
        <Skeleton className="h-6 w-40" />
        <Skeleton className="hidden h-5 w-72 md:block" />
        <Skeleton className="size-10 rounded-full" />
      </div>
      {variant === "chat" ? (
        <div aria-hidden className="grid flex-1 gap-4 px-4 pb-6 md:px-8 lg:grid-cols-[260px_1fr] xl:grid-cols-[260px_1fr_280px]">
          <div className="hidden flex-col gap-3 lg:flex">
            {Array.from({ length: 6 }, (_, i) => (
              <Skeleton key={i} className="h-16 w-full rounded-xl" />
            ))}
          </div>
          <div className="flex flex-col gap-4 rounded-xl border border-gold/30 p-4">
            <Skeleton className="h-14 w-full rounded-lg" />
            <Skeleton className="h-16 w-2/3 rounded-xl" />
            <Skeleton className="ml-auto h-12 w-1/2 rounded-xl" />
            <Skeleton className="h-20 w-3/5 rounded-xl" />
            <Skeleton className="mt-auto h-11 w-full rounded-lg" />
          </div>
          <div className="hidden flex-col gap-3 xl:flex">
            <Skeleton className="h-8 w-2/3" />
            <SkeletonText lines={4} />
          </div>
        </div>
      ) : (
        <div aria-hidden className="mx-auto flex w-full max-w-5xl flex-col gap-6 px-6 py-12">
          <Skeleton className="h-10 w-1/2" />
          <SkeletonText lines={4} />
          <div className="grid gap-4 md:grid-cols-3">
            <Skeleton className="h-40 rounded-xl" />
            <Skeleton className="h-40 rounded-xl" />
            <Skeleton className="h-40 rounded-xl" />
          </div>
        </div>
      )}
    </div>
  );
}
