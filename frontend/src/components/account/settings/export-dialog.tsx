"use client";

import * as React from "react";
import { Download, FileArchive } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { exportsApi, type DataExport } from "@/lib/api/account";
import { toApiError } from "@/lib/api/errors";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { passwordConfirmSchema, type PasswordConfirmValues } from "@/lib/schemas/settings";
import { usePagedList } from "@/lib/use-paged-list";
import { announce } from "@/components/feedback/live-region";
import { ErrorState } from "@/components/feedback/error-state";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Form, FormRootError } from "@/components/ui/form";
import { CursorPager } from "@/components/ui/pagination";
import { PasswordInput } from "@/components/ui/password-input";
import { SkeletonList } from "@/components/ui/skeleton";

import { dialogSubmit } from "./dialog-form";
import { formatBytes, formatDateTime, formatDay } from "./format";
import { PasswordNotSetNotice } from "./password-not-set";
import { SettingsField } from "./settings-ui";

const STATUS: Record<DataExport["status"], string> = {
  queued: "Queued",
  running: "Preparing",
  ready: "Ready",
  failed: "Failed",
  expired: "Expired",
};

const inProgress = (e: DataExport) => e.status === "queued" || e.status === "running";

/** Save a Blob under a file name (blob URL + temporary link). */
export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.rel = "noopener";
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
}

/** Poll an export until it leaves queued/running (2 s, backing off to 10 s). */
function usePollExport(id: string | undefined, enabled: boolean, onUpdate: (e: DataExport) => void) {
  const update = React.useRef(onUpdate);
  React.useEffect(() => {
    update.current = onUpdate;
  });
  React.useEffect(() => {
    if (!id || !enabled) return;
    let delay = 2000;
    let timer = 0;
    const ctrl = new AbortController();
    const tick = async () => {
      try {
        const fresh = await exportsApi.get(id, ctrl.signal);
        if (ctrl.signal.aborted) return;
        update.current(fresh);
        if (inProgress(fresh)) schedule();
      } catch (err) {
        if (!ctrl.signal.aborted && toApiError(err).code !== "aborted") schedule();
      }
    };
    const schedule = () => {
      timer = window.setTimeout(() => void tick(), delay);
      delay = Math.min(Math.round(delay * 1.5), 10_000);
    };
    schedule();
    return () => {
      ctrl.abort();
      window.clearTimeout(timer);
    };
  }, [id, enabled]);
}

/**
 * Download Your Data (GDPR art. 15/20): request a ZIP of the account's data
 * (password required), follow its preparation, then download it with a
 * single-use link. Exports stay available for 7 days.
 */
export function ExportDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { text, locale } = useI18n();
  const toast = useToast();
  const form = useSchemaForm(passwordConfirmSchema, { defaultValues: { password: "" } });
  const list = usePagedList((cursor, signal) => exportsApi.list(cursor, 5, signal), [open], { enabled: open });
  const [downloading, setDownloading] = React.useState<string | null>(null);
  const [noPassword, setNoPassword] = React.useState(false);
  const active = list.items.find(inProgress);

  usePollExport(active?.id, open, (fresh) => {
    list.replaceItem((e) => e.id === fresh.id, fresh);
    if (fresh.status === "ready") announce(text("Your data export is ready to download."));
    if (fresh.status === "failed") announce(text("We couldn't prepare your export. Please request a new one."));
  });

  function close(next: boolean) {
    if (!next) {
      form.reset({ password: "" });
      setNoPassword(false);
    }
    onOpenChange(next);
  }

  async function request(values: PasswordConfirmValues) {
    setNoPassword(false);
    try {
      const created = await exportsApi.request(values.password);
      list.prependItem(created);
      form.reset({ password: "" });
      announce(text("We're preparing your export. This usually takes a few minutes."));
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "password_not_set") {
        setNoPassword(true);
        return;
      }
      if (e.code === "export_in_progress") list.reset();
      if (e.code === "limit_reached") {
        form.setError("root.server", { type: e.code, message: text("You've reached today's export limit. Please try again later.") });
        return;
      }
      applyApiError(form, err, { text });
    }
  }

  async function download(item: DataExport, values: PasswordConfirmValues) {
    setDownloading(item.id);
    setNoPassword(false);
    try {
      const link = await exportsApi.downloadToken(item.id, values.password);
      const blob = await exportsApi.download(item.id, link.token);
      saveBlob(blob, `staryield-export-${item.requested_at.slice(0, 10)}.zip`);
      form.reset({ password: "" });
      toast.show({ tone: "success", message: text("Your download has started.") });
      list.replaceItem((e) => e.id === item.id, { ...item, download_count: item.download_count + 1 });
    } catch (err) {
      if (toApiError(err).code === "password_not_set") setNoPassword(true);
      else applyApiError(form, err, { text });
    } finally {
      setDownloading(null);
    }
  }

  const pending = form.formState.isSubmitting;

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent size="lg" className="settings-dialog" data-testid="export-dialog">
        <DialogHeader>
          <DialogTitle>{text("Download your data")}</DialogTitle>
          <DialogDescription>
            {text("Get a copy of your Staryield data as a ZIP file: profile, settings, chats, purchases, reviews and requests, with your images.")}
          </DialogDescription>
        </DialogHeader>
        <p className="settings-note">{text("Notes advisors wrote about you are not included. You can ask our support team for them.")}</p>
        <Form {...form}>
          <form onSubmit={(event) => dialogSubmit(form, request)(event)} noValidate className="settings-dialog-form">
            {noPassword ? <PasswordNotSetNotice /> : null}
            <FormRootError />
            <SettingsField<PasswordConfirmValues, "password">
              name="password"
              label={text("Confirm your password")}
              description={text("Needed to request or download an export.")}
              showValid={false}
            >
              {(field) => (
                <PasswordInput
                  ref={field.ref}
                  name={field.name}
                  value={field.value}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  autoComplete="current-password"
                  maxLength={128}
                  className="settings-input"
                />
              )}
            </SettingsField>

            <div className="settings-exports" aria-live="polite">
              <h3 className="settings-dialog-subtitle">{text("Your exports")}</h3>
              {list.error ? (
                <ErrorState error={list.error} onRetry={list.reload} compact />
              ) : list.loading && list.items.length === 0 ? (
                <div aria-busy="true">
                  <p role="status" className="sr-only">
                    {text("Loading your exports…")}
                  </p>
                  <SkeletonList rows={1} rowClassName="h-14" />
                </div>
              ) : list.items.length === 0 ? (
                <p className="settings-empty">{text("You haven't requested an export yet.")}</p>
              ) : (
                <ul className="settings-list" data-testid="exports-list">
                  {list.items.map((item) => (
                    <li key={item.id} className="settings-row settings-export" data-status={item.status} data-testid="export-row">
                      <span className="settings-row-main">
                        <FileArchive aria-hidden className="size-5 shrink-0" />
                        <span className="grid gap-0.5">
                          <span className="settings-row-label">{text("Requested {value0}", { value0: formatDateTime(item.requested_at, locale) })}</span>
                          <span className="settings-row-hint">
                            {item.status === "ready" && item.expires_at
                              ? text("{value0} · available until {value1}", {
                                  value0: item.bytes ? formatBytes(item.bytes, locale) : text("ZIP"),
                                  value1: formatDay(item.expires_at, locale),
                                })
                              : item.status === "failed"
                                ? text("We couldn't prepare this export. Please request a new one.")
                                : item.status === "expired"
                                  ? text("This export has expired.")
                                  : text("Preparing… This usually takes a few minutes. We'll also email you.")}
                          </span>
                        </span>
                      </span>
                      <span className="settings-export-side">
                        <span className="settings-status" data-status={item.status}>
                          {text(STATUS[item.status])}
                        </span>
                        {item.status === "ready" ? (
                          <Button
                            type="button"
                            size="lg"
                            className="settings-button"
                            disabled={downloading !== null || pending}
                            aria-busy={downloading === item.id}
                            onMouseDown={keepFocusOnPress}
                            onClick={() => void form.handleSubmit((values) => download(item, values))()}
                            data-testid="export-download"
                          >
                            <Download aria-hidden />
                            {text(downloading === item.id ? "Downloading…" : "Download")}
                          </Button>
                        ) : null}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
              <CursorPager
                page={list.page}
                hasPrev={list.hasPrev}
                hasNext={list.hasNext}
                onPrev={list.prev}
                onNext={list.next}
                loading={list.loading}
                label={text("Exports pages")}
              />
            </div>

            <DialogFooter>
              <DialogClose asChild>
                <Button type="button" variant="outline" size="xl" className="border-ink/40">
                  {text("Close")}
                </Button>
              </DialogClose>
              <Button
                type="submit"
                variant="cta"
                size="xl"
                disabled={pending || Boolean(active) || downloading !== null}
                aria-busy={pending}
                onMouseDown={keepFocusOnPress}
                data-testid="export-request"
              >
                {text(active ? "Export in progress…" : pending ? "Requesting…" : "Request a new export")}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
