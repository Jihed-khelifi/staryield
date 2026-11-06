// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import * as yup from "yup";

import { ApiError } from "@/lib/api/errors";
import type { TranslateCopy } from "@/i18n/translate-copy";
import { useI18n } from "@/i18n/i18n-provider";
import { applyApiError } from "@/lib/forms/server-errors";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { emailRule, trimmedText } from "@/lib/schemas/common";
import { FieldRow } from "@/components/ui/field";
import { Form, FormRootError } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { axeViolations, renderWithI18n } from "@/test/render";

afterEach(cleanup);

const schema = (text: TranslateCopy) =>
  yup.object({ email: emailRule(text), name: trimmedText(text, { max: 10 }) });

let serverError: unknown = null;

function ProfileForm() {
  const { text } = useI18n();
  const form = useSchemaForm(schema, { defaultValues: { email: "", name: "" } });
  return (
    <Form {...form}>
      <form
        noValidate
        onSubmit={form.handleSubmit(() => {
          if (serverError) applyApiError(form, serverError, { text });
        })}
      >
        <FormRootError />
        <FieldRow control={form.control} name="email" label="Email Address">
          {(field) => <Input type="email" {...field} />}
        </FieldRow>
        <FieldRow control={form.control} name="name" label="Full Name" maxLength={10}>
          {(field) => <Input {...field} />}
        </FieldRow>
        <button type="submit">Save</button>
      </form>
    </Form>
  );
}

describe("FieldRow + useSchemaForm", () => {
  it("shows invalid state after blur and valid state once fixed", async () => {
    renderWithI18n(<ProfileForm />);
    const email = screen.getByLabelText("Email Address");
    fireEvent.change(email, { target: { value: "jane" } });
    fireEvent.blur(email);
    await waitFor(() => expect(email.getAttribute("aria-invalid")).toBe("true"));
    expect(screen.getByText("Enter a valid email address.")).toBeTruthy();
    const item = email.closest('[data-slot="field"]')!;
    expect(item.hasAttribute("data-invalid")).toBe(true);
    expect(email.getAttribute("aria-describedby")).toContain(screen.getByText("Enter a valid email address.").id);

    fireEvent.change(email, { target: { value: "jane@example.com" } });
    await waitFor(() => expect(email.getAttribute("aria-invalid")).toBe("false"));
    expect(item.hasAttribute("data-valid")).toBe(true);
    expect(screen.getAllByText("Looks good").length).toBeGreaterThan(0);
    expect(await axeViolations()).toEqual([]);
  });

  it("counts characters and blocks submit with focus on the first error", async () => {
    renderWithI18n(<ProfileForm />);
    const name = screen.getByLabelText("Full Name");
    fireEvent.change(name, { target: { value: "Cassandra Moon" } });
    expect(screen.getByText("14/10")).toBeTruthy();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });
    await waitFor(() => expect(document.activeElement).toBe(screen.getByLabelText("Email Address")));
    expect(screen.getByText("Enter your email address.")).toBeTruthy();
    expect(screen.getByText("Use at most 10 characters.")).toBeTruthy();
  });

  it("renders server field errors and form-level errors", async () => {
    serverError = new ApiError({
      code: "validation_failed",
      message: "x",
      status: 422,
      details: { fields: { email: "This email is already used." } },
    });
    renderWithI18n(<ProfileForm />);
    fireEvent.change(screen.getByLabelText("Email Address"), { target: { value: "jane@example.com" } });
    fireEvent.change(screen.getByLabelText("Full Name"), { target: { value: "Jane" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });
    await waitFor(() => expect(screen.getByText("This email is already used.")).toBeTruthy());

    serverError = new ApiError({ code: "service_unavailable", message: "down", status: 503 });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain("The service is temporarily unavailable"),
    );
    serverError = null;
  });
});
