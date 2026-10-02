"use client";
import { useState } from "react";
import { Upload } from "lucide-react";
import Link from "next/link";
import { useI18n } from "@/i18n/i18n-provider";
import { PublicFrame } from "@/components/layout/public-frame";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

export function ContactForm() {
  const { text, locale } = useI18n();
  const [description, setDescription] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [error, setError] = useState("");
  function selectFiles(selected: File[]) {
    if (
      selected.length > 5 ||
      selected.reduce((total, file) => total + file.size, 0) > 45 * 1024 * 1024
    ) {
      setError("Choose up to 5 files totaling no more than 45 MB.");
      return;
    }
    if (
      selected.some(
        (file) =>
          !["image/svg+xml", "image/png", "image/jpeg", "image/gif"].includes(
            file.type,
          ),
      )
    ) {
      setError("Choose SVG, PNG, JPG, or GIF files.");
      return;
    }
    setFiles(selected);
    setError("");
  }
  return (
    <PublicFrame>
      <section className="contact-page">
        <p className="eyebrow text-center">{text("Support portal")}</p>
        <h1 className="mt-4 text-center font-display text-[44px]">
          {text("Contact us ")}
        </h1>
        <div className="mx-auto my-6 h-px w-24 bg-gold" />
        <form
          className="contact-card paper-page"
          onSubmit={(event) => {
            event.preventDefault();
            setError(
              "Your request has not been sent. The support service is not connected yet.",
            );
          }}
        >
          <p className="mb-7 border-b border-gold/30 pb-6 font-light leading-relaxed">
            {text(
              "Have questions about your spiritual readings, natal chart accuracy, or billing? Our dedicated support circle is here to offer guidance and align your technical path. ",
            )}
          </p>
          <label className="field">
            {text("Subject")}{" "}
            <Input
              placeholder={text("Enter your subject")}
              required
              name="subject"
            />
          </label>
          <label className="field mt-6">
            {text("Email")}{" "}
            <Input
              type="email"
              autoComplete="email"
              name="email"
              placeholder={text("example@email.com")}
              required
            />
          </label>
          <label className="field mt-6">
            {text("Describe your issue ")}
            <Textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder={text("Describe your issue in detail…")}
              minLength={10}
              maxLength={5000}
              name="description"
              required
              className="min-h-36 resize-y bg-white"
            />
            <span className="text-xs font-light">
              {description.length}
              {text(
                "/5000 — Please describe your situation in as much detail as possible so we can help you quickly and accurately. ",
              )}
            </span>
          </label>
          <div className="mt-6">
            <p className="mb-2 text-sm">{text("Attachments (optional)")}</p>
            <label
              className="attachment-dropzone"
              onDragOver={(event) => event.preventDefault()}
              onDrop={(event) => {
                event.preventDefault();
                selectFiles(Array.from(event.dataTransfer.files));
              }}
            >
              <Upload size={20} className="mx-auto text-gold" />
              <p className="mt-3">
                <span className="text-amber underline">
                  {text("Click to upload")}
                </span>{" "}
                {text("or drag and drop ")}
              </p>
              <p className="mt-3 text-xs font-light">
                {text("SVG, PNG, JPG, GIF (max. 45MB total) ")}
              </p>
              <input
                className="sr-only"
                type="file"
                multiple
                accept="image/svg+xml,image/png,image/jpeg,image/gif"
                onChange={(event) =>
                  selectFiles(Array.from(event.target.files ?? []))
                }
              />
            </label>
            <p className="mt-2 text-xs font-light">
              {text("Upload files to support your request (Maximum 5 files) ")}
            </p>
            {files.map((file) => (
              <div
                key={file.name}
                className="mt-2 flex justify-between gap-4 text-sm"
              >
                <span className="truncate">{text(file.name)}</span>
                <button
                  type="button"
                  aria-label={text("Remove {value0}", { value0: file.name })}
                  onClick={() =>
                    setFiles(files.filter((item) => item !== file))
                  }
                >
                  {text("Remove ")}
                </button>
              </div>
            ))}
          </div>
          <label className="mt-7 flex items-start gap-3 text-xs">
            <input type="checkbox" required />
            <span>
              {text("I accept Staryield’s")}{" "}
              <Link className="underline" href={`/${locale}/terms`}>
                {text("Terms of Use ")}
              </Link>{" "}
              {text("and")}{" "}
              <Link className="underline" href={`/${locale}/privacy`}>
                {text("Privacy Policy ")}
              </Link>{" "}
              {text(
                "to the extent they apply to the processing of this request. ",
              )}
            </span>
          </label>
          <p role="alert" className="mt-4 text-sm text-red-800">
            {text(error)}
          </p>
          <Button
            type="submit"
            className="gold-button mx-auto mt-6 flex h-12 w-full max-w-sm"
          >
            {text("Submit ")}
          </Button>
        </form>
      </section>
    </PublicFrame>
  );
}
