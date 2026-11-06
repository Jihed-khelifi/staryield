import type * as React from "react";
import type { FieldValues, SubmitHandler, UseFormReturn } from "react-hook-form";

/**
 * onSubmit for a form rendered in a dialog portal. React propagates
 * synthetic events through portals to the React ancestors, so without
 * stopPropagation the settings form around the dialog's trigger would also
 * receive this submit.
 */
export function dialogSubmit<T extends FieldValues>(form: Pick<UseFormReturn<T>, "handleSubmit">, onValid: SubmitHandler<T>) {
  return (event: React.FormEvent<HTMLFormElement>) => {
    event.stopPropagation();
    void form.handleSubmit(onValid)(event);
  };
}
