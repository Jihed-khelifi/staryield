// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render } from "@testing-library/react";

vi.mock("@/components/layout/site-header", () => ({
  SiteHeader: () => <header data-testid="header" />,
  MobileNavigation: () => <nav data-testid="mobile-nav" />,
}));

import { AppFrame } from "@/components/layout/app-frame";

afterEach(() => cleanup());

describe("AppFrame", () => {
  it("shows the bottom navigation and reserves room for it by default", () => {
    const { getByTestId, container } = render(<AppFrame>x</AppFrame>);
    expect(getByTestId("mobile-nav")).toBeTruthy();
    const frame = container.firstElementChild as HTMLElement;
    expect(frame.className).toContain("max-md:pb-[88px]");
    expect(frame.className).toContain("max-w-[1440px]");
    expect(frame.className).not.toContain("chat-frame");
  });

  it("chatroom: no bottom navigation, no reserved padding, wide, visible-area frame", () => {
    const { queryByTestId, container } = render(
      <AppFrame fill wide mobileNav={false}>
        x
      </AppFrame>,
    );
    expect(queryByTestId("mobile-nav")).toBeNull();
    const frame = container.firstElementChild as HTMLElement;
    expect(frame.className).not.toContain("max-md:pb-[88px]");
    expect(frame.className).toContain("max-w-[2560px]");
    expect(frame.className).toContain("chat-frame");
    expect(frame.className).toContain("h-dvh");
    expect(frame.getAttribute("data-mobile-nav")).toBe("off");
  });
});

describe("AppFrame phone header", () => {
  it("renders the site header on phones by default", () => {
    const { queryByTestId } = render(<AppFrame>x</AppFrame>);
    expect(queryByTestId("site-header-md-up")).toBeNull();
    expect(queryByTestId("header")).toBeTruthy();
  });

  it("phoneHeader={false} keeps the header for tablets and up only", () => {
    const { getByTestId } = render(<AppFrame phoneHeader={false}>x</AppFrame>);
    const wrapper = getByTestId("site-header-md-up");
    expect(wrapper.className).toContain("max-md:hidden");
    expect(wrapper.className).toContain("contents");
    expect(wrapper.querySelector("[data-testid=header]")).toBeTruthy();
  });
});
