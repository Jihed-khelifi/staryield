"use client"
/* eslint-disable @next/next/no-img-element */

import * as React from "react"
import { Clock, CheckCircle2, ImageOff, Loader2, XCircle } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { ADVISOR_PHOTO_MAX_BYTES, advisorApi } from "@/lib/api/endpoints"
import { errorMessage, toApiError } from "@/lib/api/errors"
import type { AppNotification, PhotoState, PhotoSubmission } from "@/lib/api/types"
import { useRealtimeEvent } from "@/lib/auth/session-provider"
import { formatDate } from "@/lib/format"
import { useAsync } from "@/lib/use-async"
import { announce } from "@/components/feedback/live-region"
import { ErrorState } from "@/components/feedback/error-state"
import { useToast } from "@/components/feedback/toast-provider"
import { Skeleton } from "@/components/ui/skeleton"
import { PhotoCropDialog } from "@/components/psychic/settings/photo-crop-dialog"
import { ACCEPTED_TYPES, isAcceptedType } from "@/components/psychic/settings/photo-crop"

/** Loads the advisor's own pending or rejected photo (private) as an object URL. */
function usePrivatePhoto(submission: PhotoSubmission | null | undefined) {
  const id = submission && submission.has_image && submission.status !== "approved" ? submission.id : null
  const [result, setResult] = React.useState<{ id: string; url: string | null; failed: boolean } | null>(null)
  React.useEffect(() => {
    if (!id) return
    const controller = new AbortController()
    let url: string | null = null
    advisorApi.photoImage(controller.signal).then(
      (blob) => {
        if (controller.signal.aborted) return
        url = URL.createObjectURL(blob)
        setResult({ id, url, failed: false })
      },
      () => {
        if (!controller.signal.aborted) setResult({ id, url: null, failed: true })
      }
    )
    return () => {
      controller.abort()
      if (url) URL.revokeObjectURL(url)
    }
  }, [id])
  return result && result.id === id ? result : null
}

const statusIcon = { pending: Clock, rejected: XCircle, approved: CheckCircle2 } as const

/** "Pending review" / "Not approved" / "Approved" chip (icon + text, never colour alone). */
function ReviewChip({ status }: { status: PhotoSubmission["status"] }) {
  const { text } = useI18n()
  const Icon = statusIcon[status]
  const label = status === "pending" ? "Pending review" : status === "rejected" ? "Not approved" : "Photo approved"
  return (
    <span className="photo-review-chip" data-status={status}>
      <Icon aria-hidden className="size-3.5" />
      {text(label)}
    </span>
  )
}

/**
 * Profile picture (Figma 1219:1684): the live photo, the face-guidance hint
 * and "change". A new photo is cropped to a square in the browser, then
 * sent for review; it stays private (and the current photo stays live)
 * until the Staryield team approves it. Pending and declined photos are
 * shown with their status and the reason.
 */
export function ProfilePhoto({ name }: { name: string }) {
  const { text, locale } = useI18n()
  const toast = useToast()
  const photo = useAsync((signal) => advisorApi.photo(signal), [])
  const { reload, setData } = photo
  const [file, setFile] = React.useState<File | null>(null)
  const [busy, setBusy] = React.useState<"upload" | "withdraw" | null>(null)
  const [problem, setProblem] = React.useState("")
  const inputRef = React.useRef<HTMLInputElement>(null)
  const problemId = React.useId()
  const hintId = React.useId()

  // A review decision arrives as an in-app notification.
  useRealtimeEvent<{ notification?: AppNotification }>("notification.created", (data) => {
    const kind = data?.notification?.kind
    if (kind === "photo.approved" || kind === "photo.rejected") reload()
  })

  const submission = photo.data?.submission ?? null
  const preview = usePrivatePhoto(submission)

  function pick(event: React.ChangeEvent<HTMLInputElement>) {
    const picked = event.target.files?.[0] ?? null
    event.target.value = "" // picking the same file again still triggers a change
    setProblem("")
    if (!picked) return
    if (!isAcceptedType(picked.type)) {
      setProblem(text("Use a PNG, JPEG or WebP image."))
      return
    }
    if (picked.size > ADVISOR_PHOTO_MAX_BYTES) {
      setProblem(text("Choose an image smaller than {value0} MB.", { value0: ADVISOR_PHOTO_MAX_BYTES / (1024 * 1024) }))
      return
    }
    setFile(picked)
  }

  async function upload(blob: Blob) {
    setBusy("upload")
    try {
      const next: PhotoState = await advisorApi.uploadPhoto(blob, "photo.jpg")
      setData(next)
      setFile(null)
      toast.show({ tone: "success", message: text("Photo sent for review.") })
      announce(text("Your new photo is waiting for review."))
    } catch (err) {
      const e = toApiError(err)
      setFile(null)
      setProblem(e.code === "validation_failed" && e.fields.photo ? e.fields.photo : text(errorMessage(e)))
    } finally {
      setBusy(null)
    }
  }

  async function withdraw() {
    setBusy("withdraw")
    setProblem("")
    try {
      await advisorApi.withdrawPhoto()
      setData((current) => (current ? { ...current, submission: null } : current))
      reload()
      toast.show({ tone: "info", message: text("Photo withdrawn.") })
    } catch (err) {
      toast.showError(err)
    } finally {
      setBusy(null)
    }
  }

  if (photo.error && !photo.data) {
    return <ErrorState error={photo.error} onRetry={reload} compact title={text("Your photo could not be loaded.")} />
  }
  if (!photo.data) {
    return (
      <div className="photo-field" aria-busy="true" data-testid="photo-loading">
        <p role="status" className="sr-only">
          {text("Loading your photo…")}
        </p>
        <Skeleton className="photo-frame" />
        <div className="grid flex-1 content-start gap-2">
          <Skeleton className="h-3.5 w-full" />
          <Skeleton className="h-3.5 w-2/3" />
          <Skeleton className="mt-6 h-9 w-24" />
        </div>
      </div>
    )
  }

  const { image_url: live } = photo.data
  const pending = submission?.status === "pending" ? submission : null
  const rejected = submission?.status === "rejected" ? submission : null
  // The submitted (private) photo replaces the live one in the frame while
  // it is pending or declined; the live photo is shown otherwise.
  const wantsPreview = Boolean((pending || rejected) && submission?.has_image)
  const previewUrl = wantsPreview ? (preview?.url ?? null) : null
  const previewLoading = wantsPreview && !preview
  const shown = previewUrl ?? live

  return (
    <div className="grid gap-2" data-testid="profile-photo">
      <div className="photo-field">
        <div className="photo-frame" data-state={previewUrl ? (pending ? "pending" : "rejected") : "live"}>
          {previewLoading ? (
            <Skeleton className="size-full rounded-none" />
          ) : shown ? (
            <img
              src={shown}
              alt={
                previewUrl && pending
                  ? text("Your new photo, waiting for review")
                  : previewUrl
                    ? text("Your photo that was not approved")
                    : text("Profile photo of {value0}", { value0: name })
              }
              className="size-full object-cover"
              data-testid="photo-image"
            />
          ) : (
            <span className="photo-frame-empty">
              <ImageOff aria-hidden className="size-6" />
              <span className="sr-only">{text("No profile photo yet")}</span>
            </span>
          )}
        </div>
        <div className="photo-side">
          <p id={hintId} className="photo-hint">
            {text("Make sure your face is clearly displayed in the upper half of the image")}
          </p>
          {submission ? (
            <div className="grid gap-1.5" data-testid="photo-review">
              <ReviewChip status={submission.status} />
              {pending ? (
                <p className="photo-note">{text("Your current photo stays on your profile until this one is approved.")}</p>
              ) : rejected ? (
                <p className="photo-note" data-testid="photo-reason">
                  <span className="font-semibold">{text("Reason:")}</span> {rejected.reason}
                </p>
              ) : (
                <p className="photo-note">
                  {text("Approved on {value0}.", { value0: formatDate(submission.reviewed_at ?? submission.submitted_at, locale) })}
                </p>
              )}
            </div>
          ) : null}
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              className="photo-button"
              onClick={() => inputRef.current?.click()}
              disabled={busy !== null}
              aria-describedby={`${hintId}${problem ? ` ${problemId}` : ""}`}
              data-testid="photo-change"
            >
              {busy === "upload" ? <Loader2 aria-hidden className="size-4 animate-spin motion-reduce:animate-none" /> : null}
              {text(pending || rejected ? "Upload another photo" : live ? "Change photo" : "Upload a photo")}
            </button>
            {pending ? (
              <button
                type="button"
                className="photo-button"
                onClick={() => void withdraw()}
                disabled={busy !== null}
                data-testid="photo-withdraw"
              >
                {busy === "withdraw" ? <Loader2 aria-hidden className="size-4 animate-spin motion-reduce:animate-none" /> : null}
                {text("Withdraw")}
              </button>
            ) : null}
          </div>
          <input
            ref={inputRef}
            type="file"
            accept={ACCEPTED_TYPES.join(",")}
            className="hidden"
            tabIndex={-1}
            aria-hidden
            onChange={pick}
            data-testid="photo-input"
          />
        </div>
      </div>
      <p className="photo-note">
        {text("Square crop, PNG, JPEG or WebP up to 5 MB. Photos are checked by our team before they appear on your profile.")}
      </p>
      <p id={problemId} role="alert" className="field-message" data-tone={problem ? "error" : undefined} hidden={!problem}>
        {problem}
      </p>
      <PhotoCropDialog file={file} busy={busy === "upload"} onCancel={() => setFile(null)} onConfirm={(blob) => void upload(blob)} />
    </div>
  )
}
