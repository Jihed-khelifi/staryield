"use client"

import * as React from "react"
import { yupResolver } from "@hookform/resolvers/yup"
import { useForm } from "react-hook-form"
import { FiSend } from "react-icons/fi"

import { useI18n } from "@/i18n/i18n-provider"
import {
  createMessageSchema,
  type MessageFormValues,
} from "@/lib/schemas/message-schema"
import { Button } from "@/components/ui/button"
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"
import { Input } from "@/components/ui/input"

export function MessageComposer({
  onSend,
  note,
}: {
  onSend: (message: string) => void
  note: string
}) {
  const { t } = useI18n()
  const schema = React.useMemo(() => createMessageSchema(t), [t])

  const form = useForm<MessageFormValues>({
    resolver: yupResolver(schema),
    defaultValues: { message: "" },
  })

  function handleSubmit(values: MessageFormValues) {
    onSend(values.message.trim())
    form.reset({ message: "" })
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(handleSubmit)}
        className="flex flex-col gap-1.5 border-t border-border p-3"
      >
        <FormField
          control={form.control}
          name="message"
          render={({ field }) => (
            <FormItem className="gap-1.5">
              <FormLabel className="sr-only">
                {t("chatroom.composer.label")}
              </FormLabel>
              <div className="flex items-center gap-2">
                <FormControl>
                  <Input
                    {...field}
                    autoComplete="off"
                    enterKeyHint="send"
                    placeholder={t("chatroom.composer.placeholder")}
                    className="h-auto rounded-lg border-0 bg-warm-gray px-2.5 py-2.5 font-ui text-sm text-ink shadow-none placeholder:text-ink/60 md:text-sm"
                  />
                </FormControl>
                <Button
                  type="submit"
                  size="icon"
                  variant="ghost"
                  aria-label={t("chatroom.composer.send")}
                  className="shrink-0 text-ink hover:bg-warm-gray sm:hidden"
                >
                  <FiSend className="size-4" aria-hidden />
                </Button>
              </div>
              <FormMessage className="font-ui text-[11px]" />
            </FormItem>
          )}
        />

        <p className="font-ui text-[10px] text-ink opacity-50">{note}</p>
      </form>
    </Form>
  )
}
