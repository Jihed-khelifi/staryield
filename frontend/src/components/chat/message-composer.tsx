"use client";

import * as React from "react";
import { yupResolver } from "@hookform/resolvers/yup";
import { useForm } from "react-hook-form";
import { FiSend } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { createMessageSchema, type MessageFormValues } from "@/lib/schemas/message-schema";
import { Button } from "@/components/ui/button";
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import { Input } from "@/components/ui/input";

/** Typing indicators: announce at most every 2s, stop after 3s idle. */
const TYPING_EVERY_MS = 2000;
const TYPING_IDLE_MS = 3000;

export function MessageComposer({
  onSend,
  onTyping,
  note,
  disabled = false,
  disabledReason,
}: {
  onSend: (message: string) => void;
  onTyping?: (typing: boolean) => void;
  note: string;
  disabled?: boolean;
  disabledReason?: string;
}) {
  const { t } = useI18n();
  const schema = React.useMemo(() => createMessageSchema(t), [t]);
  const lastTyping = React.useRef(0);
  const idleTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  const form = useForm<MessageFormValues>({
    resolver: yupResolver(schema),
    defaultValues: { message: "" },
  });

  const stopTyping = React.useCallback(() => {
    if (idleTimer.current) clearTimeout(idleTimer.current);
    idleTimer.current = null;
    if (lastTyping.current) {
      lastTyping.current = 0;
      onTyping?.(false);
    }
  }, [onTyping]);

  React.useEffect(() => stopTyping, [stopTyping]);

  function typed() {
    if (!onTyping || disabled) return;
    const now = Date.now();
    if (now - lastTyping.current > TYPING_EVERY_MS) {
      lastTyping.current = now;
      onTyping(true);
    }
    if (idleTimer.current) clearTimeout(idleTimer.current);
    idleTimer.current = setTimeout(stopTyping, TYPING_IDLE_MS);
  }

  function handleSubmit(values: MessageFormValues) {
    stopTyping();
    onSend(values.message.trim());
    form.reset({ message: "" });
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="flex flex-col gap-1.5 border-t border-border p-3">
        <FormField
          control={form.control}
          name="message"
          render={({ field }) => (
            <FormItem className="gap-1.5">
              <FormLabel className="sr-only">{t("chatroom.composer.label")}</FormLabel>
              <div className="flex items-center gap-2">
                <FormControl>
                  <Input
                    {...field}
                    onChange={(e) => {
                      field.onChange(e);
                      typed();
                    }}
                    disabled={disabled}
                    autoComplete="off"
                    enterKeyHint="send"
                    maxLength={1000}
                    placeholder={disabled && disabledReason ? disabledReason : t("chatroom.composer.placeholder")}
                    className="h-auto rounded-lg border-0 bg-cream px-2.5 py-2.5 font-serif text-sm text-ink shadow-none placeholder:text-ink/60 md:text-sm"
                  />
                </FormControl>
                <Button
                  type="submit"
                  disabled={disabled}
                  size="icon"
                  variant="ghost"
                  aria-label={t("chatroom.composer.send")}
                  className="shrink-0 text-ink hover:bg-warm-gray"
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
  );
}
