import type { Metadata, Viewport } from "next";
import { notFound } from "next/navigation";

import { isLocale, localePath, locales } from "@/i18n/config";
import { getDictionary } from "@/i18n/dictionaries";
import { AppFrame } from "@/components/layout/app-frame";
import { ChatroomShell } from "@/components/chat/chatroom-shell";
import { ScrollbarKind } from "@/components/layout/scrollbar-kind";
import { AgeGuard } from "@/lib/auth/age-gate"; // revisions 5.0: 18+ age gate (§24.6)

type PageParams = { params: Promise<{ lang: string }> };

/**
 * Phones shrink the layout when the keyboard opens (the composer stays above
 * it) instead of panning the page; iOS Safari, which ignores this, is covered
 * by the `useVisualViewport` hook in the chatroom.
 */
export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  interactiveWidget: "resizes-content",
};

export async function generateMetadata({
  params,
}: PageParams): Promise<Metadata> {
  const { lang } = await params;
  if (!isLocale(lang)) return {};

  const dictionary = await getDictionary(lang);
  return {
    title: dictionary.meta.chatroomTitle,
    description: dictionary.meta.chatroomDescription,
    alternates: {
      languages: Object.fromEntries(
        locales.map((locale) => [locale, localePath("/chatroom", locale)]),
      ),
    },
  };
}

export default async function ChatroomPage({
  params,
  searchParams,
}: PageParams & { searchParams: Promise<{ reader?: string; review?: string }> }) {
  const { lang } = await params;
  if (!isLocale(lang)) notFound();
  const { reader, review } = await searchParams;

  return (
    <AppFrame fill wide mobileNav={false} phoneHeader={false}>
      <ScrollbarKind />
      <AgeGuard>
        <ChatroomShell
          key={reader ?? "inbox"}
          initialReader={reader && /^[a-z0-9-]{1,64}$/.test(reader) ? reader : undefined}
          // `review` opens the rating dialog (link of the chat.review_prompt notification).
          initialReview={review && /^[A-Za-z0-9-]{8,64}$/.test(review) ? review : undefined}
        />
      </AgeGuard>
    </AppFrame>
  );
}
