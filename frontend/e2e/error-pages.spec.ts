/**
 * Error pages recover. vinext 1.0 gives `error.tsx` and `global-error.tsx` the error and `reset`
 * only; the `retry` prop of the Next.js 16.3 docs is never passed. Production once showed an
 * error page whose "Try again" called that missing `retry` and threw "t is not a function".
 *
 * A malformed API answer (injected here) makes the profile's request list throw while rendering,
 * which shows the route error page. Once the API answers properly again, Try again must render the
 * page again (refresh the route, then reset the boundary) without any uncaught error.
 *
 * `npm run dev` runs Next.js, which does pass `retry`: only a run against the vinext build (served by
 * `wrangler dev`, docs/stripe-setup.md §8) exercises what production gets.
 */
import { expect, test } from "@playwright/test";

import { newClient } from "./helpers";

const REQUESTS = /\/api\/v1\/custom-requests(\?.*)?$/;

test("Try again on the route error page renders the page again once the failure clears", async ({ page }) => {
  await newClient(page, "error-retry");
  // `items` that is not a list: the request list's render throws.
  await page.route(REQUESTS, (route) => (route.request().method() === "GET" ? route.fulfill({ json: { items: 5 } }) : route.fallback()));
  await page.goto("/en/profile");
  const errorHeading = page.getByRole("heading", { name: "Something went wrong" });
  await expect(errorHeading).toBeVisible();

  const uncaught: string[] = [];
  page.on("pageerror", (error) => uncaught.push(`${error.name}: ${error.message}`));
  await page.unroute(REQUESTS);
  await page.getByRole("button", { name: "Try again" }).click();

  await expect(errorHeading).toBeHidden();
  await expect(page.getByRole("heading", { name: "Your requests" })).toBeVisible();
  expect(uncaught).toEqual([]);
});
