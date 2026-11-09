// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import { useBlockStrayFileDrops, useFileDrop, type FileDropOptions } from "@/lib/use-file-drop";

afterEach(cleanup);

function Target(props: FileDropOptions) {
  const { dragging, handlers } = useFileDrop(props);
  return (
    <div data-testid="target" data-dragging={dragging || undefined} {...handlers}>
      <span data-testid="child">child</span>
    </div>
  );
}

const png = new File([new Uint8Array([1])], "a.png", { type: "image/png" });
const files = (list: File[] = [png]) => ({ dataTransfer: { types: ["Files"], files: list, dropEffect: "none" } });
const text = { dataTransfer: { types: ["text/plain"], files: [] as File[], dropEffect: "none" } };

describe("useFileDrop", () => {
  it("shows the drop state while files are dragged over, using a depth counter so children do not flicker", () => {
    render(<Target onFiles={vi.fn()} />);
    const target = screen.getByTestId("target");
    const child = screen.getByTestId("child");
    fireEvent.dragEnter(target, files());
    expect(target.dataset.dragging).toBe("true");
    // Moving onto a child: enter child, leave parent.
    fireEvent.dragEnter(child, files());
    fireEvent.dragLeave(target, files());
    expect(target.dataset.dragging).toBe("true");
    // Leaving the child, then the whole target.
    fireEvent.dragLeave(child, files());
    expect(target.dataset.dragging).toBeUndefined();
  });

  it("only reacts to drags that carry files", () => {
    const onFiles = vi.fn();
    render(<Target onFiles={onFiles} />);
    const target = screen.getByTestId("target");
    fireEvent.dragEnter(target, text);
    expect(target.dataset.dragging).toBeUndefined();
    expect(fireEvent.dragOver(target, text)).toBe(true); // not prevented: text drags keep their behaviour
    fireEvent.drop(target, text);
    expect(onFiles).not.toHaveBeenCalled();
  });

  it("copies on dragover (prevents the default) and hands the files over on drop", () => {
    const onFiles = vi.fn();
    render(<Target onFiles={onFiles} />);
    const target = screen.getByTestId("target");
    const dt = files();
    expect(fireEvent.dragOver(target, dt)).toBe(false);
    expect(dt.dataTransfer.dropEffect).toBe("copy");
    fireEvent.dragEnter(target, files());
    expect(fireEvent.drop(target, files())).toBe(false);
    expect(onFiles).toHaveBeenCalledWith([png]);
    expect(target.dataset.dragging).toBeUndefined();
  });

  it("when disabled it shows no overlay, never navigates, and tells why on drop", () => {
    const onFiles = vi.fn();
    const onBlocked = vi.fn();
    render(<Target onFiles={onFiles} disabled onBlocked={onBlocked} />);
    const target = screen.getByTestId("target");
    fireEvent.dragEnter(target, files());
    expect(target.dataset.dragging).toBeUndefined();
    const dt = files();
    expect(fireEvent.dragOver(target, dt)).toBe(false);
    expect(dt.dataTransfer.dropEffect).toBe("none");
    expect(fireEvent.drop(target, files())).toBe(false);
    expect(onFiles).not.toHaveBeenCalled();
    expect(onBlocked).toHaveBeenCalledOnce();
  });

  it("resets when a drag ends elsewhere (window drop/dragend)", () => {
    render(<Target onFiles={vi.fn()} />);
    const target = screen.getByTestId("target");
    fireEvent.dragEnter(target, files());
    expect(target.dataset.dragging).toBe("true");
    fireEvent.dragEnd(window);
    expect(target.dataset.dragging).toBeUndefined();
  });
});

describe("useBlockStrayFileDrops", () => {
  function Page({ active = true }: { active?: boolean }) {
    useBlockStrayFileDrops(active);
    return <div data-testid="elsewhere" />;
  }

  it("stops a file dropped outside any target from navigating the browser away", () => {
    render(<Page />);
    const elsewhere = screen.getByTestId("elsewhere");
    expect(fireEvent.drop(elsewhere, files())).toBe(false);
    expect(fireEvent.dragOver(elsewhere, files())).toBe(false);
    // Text drags are left alone.
    expect(fireEvent.drop(elsewhere, text)).toBe(true);
  });

  it("does not fight a drop target that already took the drag", () => {
    render(
      <>
        <Page />
        <Target onFiles={vi.fn()} />
      </>,
    );
    const dt = files();
    fireEvent.dragOver(screen.getByTestId("target"), dt);
    expect(dt.dataTransfer.dropEffect).toBe("copy"); // the guard did not turn it into "none"
  });

  it("does nothing when inactive or unmounted", () => {
    const { unmount } = render(<Page active={false} />);
    expect(fireEvent.drop(screen.getByTestId("elsewhere"), files())).toBe(true);
    unmount();
    expect(fireEvent.drop(document.body, files())).toBe(true);
  });
});
