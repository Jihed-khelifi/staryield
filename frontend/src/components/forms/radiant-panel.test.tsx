// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen } from "@testing-library/react";

import { axeViolations, renderWithI18n } from "@/test/render";
import { RadiantPanel } from "@/components/forms/radiant-panel";

vi.mock("next/navigation", () => ({ usePathname: () => "/en/login", useRouter: () => ({ push: vi.fn(), replace: vi.fn() }) }));

afterEach(cleanup);

describe("RadiantPanel (logo pages)", () => {
  it("renders one clean ray set centred on the sun, inside the brand block", () => {
    const { container } = renderWithI18n(
      <RadiantPanel>
        <p>content</p>
      </RadiantPanel>,
    );
    const brand = container.querySelector(".radiant-brand");
    const rays = container.querySelectorAll(".radiant-rays img");
    expect(rays).toHaveLength(1);
    expect(rays[0].getAttribute("src")).toMatch(/assets\/brand\/auth-rays-[0-9a-f]+\.svg$/);
    expect(brand?.contains(rays[0])).toBe(true);
    expect(rays[0].closest(".radiant-rays")?.getAttribute("aria-hidden")).toBe("true");
  });

  it("shows the standard wordmark and the large faced sun", async () => {
    const { container } = renderWithI18n(
      <RadiantPanel>
        <p>content</p>
      </RadiantPanel>,
    );
    expect(container.querySelector(".radiant-wordmark")?.textContent?.trim().toLowerCase()).toBe("staryield");
    expect(container.querySelector(".sun-logo-large .sun-face")).not.toBeNull();
    expect(screen.getByText("content")).toBeTruthy();
    expect(await axeViolations(container)).toEqual([]);
  });
});
