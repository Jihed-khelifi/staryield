// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";

import { CursorPager, NumberedPager, pageItems } from "@/components/ui/pagination";
import { axeViolations, renderWithI18n } from "@/test/render";

afterEach(cleanup);

describe("pageItems", () => {
  it("lists every page when they all fit", () => {
    expect(pageItems(1, 5)).toEqual([1, 2, 3, 4, 5]);
    expect(pageItems(3, 7)).toEqual([1, 2, 3, 4, 5, 6, 7]);
  });
  it("collapses gaps into ellipses around the current page", () => {
    expect(pageItems(10, 20)).toEqual([1, "ellipsis-start", 9, 10, 11, "ellipsis-end", 20]);
    expect(pageItems(1, 20)).toEqual([1, 2, "ellipsis-end", 20]);
    expect(pageItems(20, 20)).toEqual([1, "ellipsis-start", 19, 20]);
    expect(pageItems(3, 20)).toEqual([1, 2, 3, 4, "ellipsis-end", 20]);
  });
  it("clamps out-of-range pages and handles empty lists", () => {
    expect(pageItems(99, 3)).toEqual([1, 2, 3]);
    expect(pageItems(1, 0)).toEqual([]);
  });
});

describe("CursorPager", () => {
  it("renders nothing for a single page", () => {
    const { container } = renderWithI18n(
      <CursorPager page={1} hasPrev={false} hasNext={false} onPrev={vi.fn()} onNext={vi.fn()} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("disables Previous on the first page and announces the page", async () => {
    const onNext = vi.fn();
    const onPrev = vi.fn();
    renderWithI18n(<CursorPager page={1} hasPrev={false} hasNext onPrev={onPrev} onNext={onNext} label="Reviews pages" />);
    expect(screen.getByRole("navigation", { name: "Reviews pages" })).toBeTruthy();
    const previous = screen.getByRole("button", { name: /previous/i }) as HTMLButtonElement;
    expect(previous.disabled).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: /next/i }));
    expect(onNext).toHaveBeenCalledOnce();
    expect(screen.getByText("Page 1")).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });

  it("shows the total when known and blocks clicks while loading", () => {
    const onNext = vi.fn();
    renderWithI18n(<CursorPager page={2} pageCount={5} hasPrev hasNext loading onPrev={vi.fn()} onNext={onNext} />);
    expect(screen.getByText("Page 2 of 5")).toBeTruthy();
    const next = screen.getByRole("button", { name: /next/i }) as HTMLButtonElement;
    expect(next.disabled).toBe(true);
  });
});

describe("NumberedPager", () => {
  it("marks the current page and changes page on click", async () => {
    const onPageChange = vi.fn();
    renderWithI18n(<NumberedPager page={2} pageCount={4} onPageChange={onPageChange} />);
    const current = screen.getByRole("button", { name: "Go to page 2" });
    expect(current.getAttribute("aria-current")).toBe("page");
    fireEvent.click(screen.getByRole("button", { name: "Go to page 3" }));
    expect(onPageChange).toHaveBeenCalledWith(3);
    fireEvent.click(current);
    expect(onPageChange).toHaveBeenCalledTimes(1);
    expect(await axeViolations()).toEqual([]);
  });

  it("renders nothing with a single page", () => {
    const { container } = renderWithI18n(<NumberedPager page={1} pageCount={1} onPageChange={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });
});
