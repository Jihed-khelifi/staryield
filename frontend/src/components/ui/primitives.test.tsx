// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Skeleton, SkeletonList } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/ui/status-badge";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

beforeAll(stubResizeObserver);
afterEach(cleanup);

describe("Dialog", () => {
  it("is a labelled modal with a blurred backdrop and a translated close button", async () => {
    const onOpenChange = vi.fn();
    renderWithI18n(
      <Dialog open onOpenChange={onOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Download your data</DialogTitle>
            <DialogDescription>We will email you a link.</DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>,
    );
    const dialog = screen.getByRole("dialog", { name: "Download your data" });
    expect(dialog.getAttribute("aria-describedby")).toBeTruthy();
    const overlay = document.querySelector('[data-slot="dialog-overlay"]');
    expect(overlay?.getAttribute("data-overlay")).toBe("blur");
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(await axeViolations()).toEqual([]);
  });

  it("can use a plain dim backdrop", () => {
    renderWithI18n(
      <Dialog open>
        <DialogContent overlay="dim" showCloseButton={false}>
          <DialogTitle>Plain</DialogTitle>
          <DialogDescription>Body</DialogDescription>
        </DialogContent>
      </Dialog>,
    );
    expect(document.querySelector('[data-slot="dialog-overlay"]')?.getAttribute("data-overlay")).toBe("dim");
    expect(screen.queryByRole("button", { name: "Close" })).toBeNull();
  });
});

describe("AlertDialog", () => {
  it("asks for confirmation with a safe default", async () => {
    const onConfirm = vi.fn();
    renderWithI18n(
      <AlertDialog open>
        <AlertDialogContent>
          <AlertDialogTitle>Close account?</AlertDialogTitle>
          <AlertDialogDescription>This cannot be undone.</AlertDialogDescription>
          <AlertDialogCancel>Keep my account</AlertDialogCancel>
          <AlertDialogAction destructive onClick={onConfirm}>
            Close account
          </AlertDialogAction>
        </AlertDialogContent>
      </AlertDialog>,
    );
    const dialog = screen.getByRole("alertdialog", { name: "Close account?" });
    expect(dialog).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Keep my account" }));
    fireEvent.click(screen.getByRole("button", { name: "Close account" }));
    expect(onConfirm).toHaveBeenCalledOnce();
    expect(await axeViolations()).toEqual([]);
  });
});

describe("form controls", () => {
  it("switch, checkbox and radio group expose their state accessibly", async () => {
    const onSwitch = vi.fn();
    renderWithI18n(
      <div>
        <Label htmlFor="email-notifications">Email Notifications</Label>
        <Switch id="email-notifications" defaultChecked onCheckedChange={onSwitch} />
        <Label htmlFor="consent">I agree</Label>
        <Checkbox id="consent" />
        <RadioGroup aria-label="Theme" defaultValue="beige">
          <RadioGroupItem value="beige" aria-label="Warm Beige" />
          <RadioGroupItem value="cream" aria-label="Light Cream" />
        </RadioGroup>
      </div>,
    );
    const toggle = screen.getByRole("switch", { name: "Email Notifications" });
    expect(toggle.getAttribute("aria-checked")).toBe("true");
    fireEvent.click(toggle);
    expect(onSwitch).toHaveBeenCalledWith(false);
    const checkbox = screen.getByRole("checkbox", { name: "I agree" });
    fireEvent.click(checkbox);
    expect(checkbox.getAttribute("aria-checked")).toBe("true");
    expect(screen.getByRole("radio", { name: "Warm Beige" }).getAttribute("aria-checked")).toBe("true");
    expect(await axeViolations()).toEqual([]);
  });

  it("password input toggles visibility without submitting", () => {
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
    renderWithI18n(
      <form onSubmit={onSubmit}>
        <Label htmlFor="pw">Current Password</Label>
        <PasswordInput id="pw" defaultValue="secret" />
      </form>,
    );
    const input = screen.getByLabelText("Current Password") as HTMLInputElement;
    expect(input.type).toBe("password");
    const toggle = screen.getByRole("button", { name: "Show password" });
    fireEvent.click(toggle);
    expect(input.type).toBe("text");
    expect(screen.getByRole("button", { name: "Hide password" }).getAttribute("aria-pressed")).toBe("true");
    expect(onSubmit).not.toHaveBeenCalled();
  });
});

describe("disclosure", () => {
  it("tabs and accordion are keyboard-accessible", async () => {
    renderWithI18n(
      <div>
        <Tabs defaultValue="overview">
          <TabsList aria-label="Client details">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="notes">Notes</TabsTrigger>
          </TabsList>
          <TabsContent value="overview">Sun in Aries</TabsContent>
          <TabsContent value="notes">No notes</TabsContent>
        </Tabs>
        <Accordion type="single" collapsible>
          <AccordionItem value="sun">
            <AccordionTrigger>Sun</AccordionTrigger>
            <AccordionContent>Core identity</AccordionContent>
          </AccordionItem>
        </Accordion>
      </div>,
    );
    expect(screen.getByRole("tab", { name: "Overview" }).getAttribute("aria-selected")).toBe("true");
    const trigger = screen.getByRole("button", { name: "Sun" });
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByText("Core identity")).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });
});

describe("display", () => {
  it("status badges always carry text", async () => {
    renderWithI18n(
      <div>
        <StatusBadge status="online" />
        <StatusBadge status="busy" />
        <StatusBadge status="offline" variant="dot" label="Last seen 2h ago" />
      </div>,
    );
    expect(screen.getByText("Online")).toBeTruthy();
    expect(screen.getByText("Busy")).toBeTruthy();
    expect(screen.getByText("Last seen 2h ago")).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });

  it("skeletons are hidden from assistive tech", () => {
    const { container } = renderWithI18n(
      <div>
        <Skeleton className="h-4" />
        <SkeletonList rows={2} />
      </div>,
    );
    for (const node of container.querySelectorAll('[data-slot="skeleton"]'))
      expect(node.closest("[aria-hidden]")).not.toBeNull();
  });

  it("button variants render the revisions 2.0 styles", () => {
    renderWithI18n(
      <div>
        <Button variant="cta">Explore Advisors</Button>
        <Button variant="glass">Join Waiting List</Button>
      </div>,
    );
    expect(screen.getByRole("button", { name: "Explore Advisors" }).className).toContain("shadow-glow");
    expect(screen.getByRole("button", { name: "Join Waiting List" }).className).toContain("glass");
  });
});
