import * as yup from "yup"

export const MESSAGE_MAX_LENGTH = 1000

type Translate = (path: string, values?: Record<string, string | number>) => string

/** Localised validation for the chat composer. */
export function createMessageSchema(t: Translate) {
  return yup.object({
    message: yup
      .string()
      .trim()
      .required(t("chatroom.composer.errors.required"))
      .max(
        MESSAGE_MAX_LENGTH,
        t("chatroom.composer.errors.max", { max: MESSAGE_MAX_LENGTH })
      ),
  })
}

export type MessageFormValues = yup.InferType<ReturnType<typeof createMessageSchema>>
