/**
 * Chat images and drag and drop (§22.4), end to end: the whole conversation is
 * the drop target (feed, panels, composer), a file dropped elsewhere never
 * navigates the browser away, and every image is prepared in the browser
 * before upload (real type sniffed from the bytes, large photos downscaled
 * to 2048 px, HEIC the browser cannot decode explained).
 *
 * Needs the §22 backend and a Chromium (it cannot decode HEIC, which the last
 * test relies on).
 */
import { expect, test, type BrowserContext, type Page } from "@playwright/test";

import { CLIENT, advisor, ensureCredits, expectImageLoaded, pngFixture, setAvailability, signedInPage } from "./helpers";

const RAMONE = advisor("ramone");
const RUN = Date.now().toString(36);
const UPLOAD = /\/chat\/(sessions|conversations)\/[^/]+\/attachments$/;

const message = (page: Page, text: string) => page.getByTestId("chat-message").filter({ hasText: text }).last();

/** Dispatch a drag event carrying a file onto the first element matching `selector`. */
async function dragFile(page: Page, type: "dragenter" | "dragover" | "dragleave" | "drop", selector: string, file?: { name: string; mime: string; base64: string }) {
  return page.evaluate(
    ({ type, selector, file }) => {
      const target = document.querySelector(selector);
      if (!target) throw new Error(`no element ${selector}`);
      const dt = new DataTransfer();
      if (file) dt.items.add(new File([Uint8Array.from(atob(file.base64), (c) => c.charCodeAt(0))], file.name, { type: file.mime }));
      const event = new DragEvent(type, { bubbles: true, cancelable: true, dataTransfer: dt });
      target.dispatchEvent(event);
      return { defaultPrevented: event.defaultPrevented, dropEffect: dt.dropEffect };
    },
    { type, selector, file },
  );
}

const png = (width: number, height: number, name = "dropped.png") => ({ name, mime: "image/png", base64: pngFixture(width, height).toString("base64") });

type SentImage = { type: string; size: number; width: number; height: number };

/**
 * Record the `image` part of every chat upload the page sends, measured in the
 * page itself: Playwright cannot read multipart bodies that carry files.
 */
async function captureUploads(page: Page) {
  await page.evaluate(() => {
    const w = window as unknown as { __uploads?: SentImage[]; __uploadsWrapped?: boolean };
    w.__uploads = [];
    if (w.__uploadsWrapped) return;
    w.__uploadsWrapped = true;
    const original = window.fetch.bind(window);
    window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
      if (/\/attachments$/.test(url) && init?.body instanceof FormData) {
        const file = init.body.get("image");
        if (file instanceof Blob) {
          const bitmap = await createImageBitmap(file);
          w.__uploads!.push({ type: file.type, size: file.size, width: bitmap.width, height: bitmap.height });
          bitmap.close();
        }
      }
      return original(input, init);
    };
  });
}

/** The last image the page uploaded (waits for one). */
async function lastUpload(page: Page): Promise<SentImage> {
  const read = () => page.evaluate(() => (window as unknown as { __uploads?: SentImage[] }).__uploads?.at(-1) ?? null);
  await expect.poll(read).not.toBeNull();
  return (await read())!;
}

test.describe.serial("chat images and drag and drop", () => {
  let clientCtx: BrowserContext;
  let psychicCtx: BrowserContext;
  let client: Page;
  let psychic: Page;

  test.beforeAll(async ({ browser }) => {
    ({ context: psychicCtx, page: psychic } = await signedInPage(browser, RAMONE.email, RAMONE.password));
    ({ context: clientCtx, page: client } = await signedInPage(browser, CLIENT.email, CLIENT.password));
    await ensureCredits(client);
    await psychic.goto("/en/profile");
    await setAvailability(psychic, "online");
    await psychic.goto("/en/chatroom");
    await client.goto(`/en/chatroom?reader=${RAMONE.slug}`);
    const start = client.getByRole("button", { name: /Start live chat/ }).first();
    await expect(start).toBeEnabled();
    await start.click();
    await psychic.getByRole("button", { name: "Accept", exact: true }).click();
    await expect(client.getByText("Live chat in progress")).toBeVisible();
    await expect(psychic.getByText("Live chat in progress")).toBeVisible();
  });

  test.afterAll(async () => {
    await client
      ?.getByRole("button", { name: /End the chat with/ })
      .click({ timeout: 3_000 })
      .catch(() => undefined);
    await psychic?.goto("/en/profile").catch(() => undefined);
    await setAvailability(psychic, "offline").catch(() => undefined);
    await clientCtx?.close();
    await psychicCtx?.close();
  });

  test("dropping an image on the feed shows the overlay and attaches it to the composer", async () => {
    await dragFile(client, "dragenter", "[data-testid=chat-feed]", png(48, 32));
    const overlay = client.getByTestId("drop-overlay");
    await expect(overlay).toBeVisible();
    await expect(overlay).toContainText(/Drop the image to send it to /);
    const over = await dragFile(client, "dragover", "[data-testid=chat-feed]", png(48, 32));
    expect(over.defaultPrevented).toBe(true);
    // Chromium ignores dropEffect on a script-built DataTransfer (not a real drag session):
    // the "copy" cursor is covered by the useFileDrop unit tests.
    const drop = await dragFile(client, "drop", "[data-testid=chat-feed]", png(48, 32));
    expect(drop.defaultPrevented).toBe(true);
    await expect(overlay).toHaveCount(0);
    const attachment = client.getByTestId("composer-attachment");
    await expect(attachment).toBeVisible();
    await expect(attachment).toContainText("dropped.png");
    // Focus moves to the caption field.
    await expect(client.getByRole("textbox", { name: "Message" })).toBeFocused();

    const upload = client.waitForResponse((r) => UPLOAD.test(r.url()) && r.request().method() === "POST");
    const box = client.getByRole("textbox", { name: "Message" });
    await box.fill(`dropped-${RUN}`);
    await box.press("Enter");
    expect((await upload).status()).toBe(201);
    for (const page of [client, psychic]) {
      await expect(message(page, `dropped-${RUN}`).locator("[data-kind=image]")).toBeVisible();
      await expectImageLoaded(page, `[data-testid=chat-message]:has-text("dropped-${RUN}") .chat-image-frame img >> nth=-1`);
    }
  });

  test("the whole conversation is the target: the composer and the header accept a drop too, and the overlay goes away on leave", async () => {
    for (const selector of ["#chat-composer-input", "[data-testid=chat-header]"]) {
      await dragFile(client, "dragenter", selector, png(24, 24, "again.png"));
      await expect(client.getByTestId("drop-overlay")).toBeVisible();
      await dragFile(client, "dragleave", selector, png(24, 24, "again.png"));
      await expect(client.getByTestId("drop-overlay")).toHaveCount(0);
    }
    await dragFile(client, "drop", "#chat-composer-input", png(24, 24, "again.png"));
    await expect(client.getByTestId("composer-attachment")).toContainText("again.png");
    await client.getByRole("button", { name: "Remove image" }).click();
    await expect(client.getByTestId("composer-attachment")).toHaveCount(0);
  });

  test("a file dropped outside the conversation does not navigate the browser away", async () => {
    const url = client.url();
    for (const selector of ["body", "main.chat-workspace", ".chat-inbox-column"]) {
      const over = await dragFile(client, "dragover", selector, png(24, 24, "stray.png"));
      expect(over.defaultPrevented, `dragover on ${selector}`).toBe(true);
      const drop = await dragFile(client, "drop", selector, png(24, 24, "stray.png"));
      expect(drop.defaultPrevented, `drop on ${selector}`).toBe(true);
    }
    expect(client.url()).toBe(url);
    await expect(client.getByRole("textbox", { name: "Message" })).toBeVisible();
    // Dragging text (not a file) is left alone.
    const text = await client.evaluate(() => {
      const dt = new DataTransfer();
      dt.setData("text/plain", "hello");
      const event = new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: dt });
      document.body.dispatchEvent(event);
      return event.defaultPrevented;
    });
    expect(text).toBe(false);
  });

  test("a 3000x2000 PNG is accepted and uploaded downscaled to 2048 px", async () => {
    await captureUploads(client);
    await client.getByTestId("attach-input").setInputFiles({ name: "big.png", mimeType: "image/png", buffer: pngFixture(3000, 2000) });
    await expect(client.getByTestId("composer-attachment")).toBeVisible();
    const box = client.getByRole("textbox", { name: "Message" });
    await box.fill(`big-${RUN}`);
    await box.press("Enter");
    const sent = await lastUpload(client);
    // An opaque PNG is re-encoded as JPEG at the 2048 px limit; it never goes up as 3000 px.
    expect(sent.type).toBe("image/jpeg");
    expect({ width: sent.width, height: sent.height }).toEqual({ width: 2048, height: 1365 });
    await expect(message(client, `big-${RUN}`).locator("[data-kind=image]")).toBeVisible();
    await expect(message(psychic, `big-${RUN}`).locator("[data-kind=image]")).toBeVisible();
    // What the server stored is within the limit too.
    const rendered = await client.locator(`[data-testid=chat-message]:has-text("big-${RUN}") .chat-image-frame img`).last().evaluate((img: HTMLImageElement) => img.naturalWidth);
    expect(rendered).toBeLessThanOrEqual(2048);
  });

  test("a file with an empty type is recognised by its bytes and uploaded", async () => {
    await captureUploads(client);
    const upload = client.waitForResponse((r) => UPLOAD.test(r.url()) && r.request().method() === "POST");
    await client.getByTestId("attach-input").setInputFiles({ name: "IMG_0001", mimeType: "", buffer: pngFixture(64, 48) });
    await expect(client.getByTestId("composer-attachment")).toContainText("IMG_0001");
    const box = client.getByRole("textbox", { name: "Message" });
    await box.fill(`notype-${RUN}`);
    await box.press("Enter");
    expect((await upload).status()).toBe(201);
    expect((await lastUpload(client)).type).toBe("image/png");
    await expect(message(psychic, `notype-${RUN}`).locator("[data-kind=image]")).toBeVisible();
  });

  test("a HEIC photo this browser cannot decode, and a file that is no image, are explained", async () => {
    const heic = Buffer.concat([Buffer.from([0, 0, 0, 24]), Buffer.from("ftypheic"), Buffer.alloc(64)]);
    await client.getByTestId("attach-input").setInputFiles({ name: "IMG_0002.HEIC", mimeType: "", buffer: heic });
    const alert = client.locator("form.composer").getByRole("alert");
    await expect(alert).toHaveText("This photo format can't be sent from this browser. Choose JPEG or PNG, or send a screenshot.");
    await expect(client.getByTestId("composer-attachment")).toHaveCount(0);

    await client.getByTestId("attach-input").setInputFiles({ name: "notes.png", mimeType: "image/png", buffer: Buffer.from("%PDF-1.7 not an image at all") });
    await expect(alert).toHaveText("This file isn't an image we can send. Choose a PNG, JPEG, WebP or GIF image.");
  });
});
