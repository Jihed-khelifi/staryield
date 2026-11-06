// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import { keepFocusOnPress } from "@/lib/forms/submit-press";

afterEach(cleanup);

describe("keepFocusOnPress", () => {
  it("keeps focus in the field on a primary-button press so the click still submits", () => {
    render(
      <form>
        <input aria-label="Email" />
        <button type="submit" onMouseDown={keepFocusOnPress}>
          Continue
        </button>
      </form>
    );
    const input = screen.getByRole("textbox", { name: "Email" });
    input.focus();
    const button = screen.getByRole("button", { name: "Continue" });
    expect(fireEvent.mouseDown(button, { button: 0 })).toBe(false); // default prevented: no blur
    expect(fireEvent.mouseDown(button, { button: 2 })).toBe(true); // context menu untouched
  });
});
