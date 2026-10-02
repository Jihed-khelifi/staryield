import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { isLocale, localePath, locales } from "@/i18n/config";
import { getDictionary } from "@/i18n/dictionaries";
import { AppFrame } from "@/components/layout/app-frame";
import { ChatroomShell } from "@/components/chat/chatroom-shell";

type PageParams = { params: Promise<{ lang: string }> };

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
}: PageParams & { searchParams: Promise<{ reader?: string }> }) {
  const { lang } = await params;
  if (!isLocale(lang)) notFound();
  const { reader } = await searchParams;

  return (
    <AppFrame fill>
      <ChatroomShell key={reader ?? "ramone"} initialReader={reader} />
    </AppFrame>
  );
}
