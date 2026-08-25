import type { Metadata } from "next"
import { notFound } from "next/navigation"

import { isLocale } from "@/i18n/config"
import { getDictionary } from "@/i18n/dictionaries"
import { AppFrame } from "@/components/layout/app-frame"
import { ChatroomShell } from "@/components/chat/chatroom-shell"

type PageParams = { params: Promise<{ lang: string }> }

export async function generateMetadata({
  params,
}: PageParams): Promise<Metadata> {
  const { lang } = await params
  if (!isLocale(lang)) return {}

  const dictionary = await getDictionary(lang)
  return {
    title: dictionary.meta.chatroomTitle,
    description: dictionary.meta.chatroomDescription,
  }
}

export default async function ChatroomPage({ params }: PageParams) {
  const { lang } = await params
  if (!isLocale(lang)) notFound()

  return (
    <AppFrame fill>
      <ChatroomShell />
    </AppFrame>
  )
}
