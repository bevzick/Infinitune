import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it } from "vitest";

import { MobileNavigation } from "./MobileNavigation";

afterEach(cleanup);

function renderMobileNavigation(authenticated = false, isAdmin = false) {
  return render(
    <MemoryRouter initialEntries={["/search"]}>
      <MobileNavigation authenticated={authenticated} isAdmin={isAdmin} />
    </MemoryRouter>,
  );
}

describe("MobileNavigation", () => {
  it("keeps the bottom bar compact and reveals a useful menu for guests", async () => {
    const user = userEvent.setup();
    renderMobileNavigation();
    const nav = screen.getByRole("navigation", { name: "Mobile navigation" });
    const toggle = within(nav).getByRole("button", { name: "More navigation" });

    expect(within(nav).getAllByRole("link")).toHaveLength(4);
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    const group = document.getElementById("mobile-more-panel") as HTMLElement;
    expect(
      within(group).getByRole("link", { name: "Sign in" }),
    ).toHaveAttribute("href", "/login");
    expect(
      within(group).getByRole("link", { name: "Register" }),
    ).toHaveAttribute("href", "/register");
    expect(
      within(group).queryByRole("link", { name: "Creator releases" }),
    ).not.toBeInTheDocument();
  });

  it("closes on Escape and restores keyboard focus to the toggle", async () => {
    const user = userEvent.setup();
    renderMobileNavigation(true);
    const toggle = screen.getByRole("button", { name: "More navigation" });
    await user.click(toggle);
    expect(
      screen.getByRole("link", { name: "Creator releases" }),
    ).toBeVisible();
    await user.keyboard("{Escape}");
    expect(toggle).toHaveFocus();
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByRole("link", { name: "Creator releases" }),
    ).not.toBeInTheDocument();
  });

  it("opens creator and admin destinations only for eligible sessions and closes on navigation", async () => {
    const user = userEvent.setup();
    renderMobileNavigation(true, true);
    const toggle = screen.getByRole("button", { name: "More navigation" });
    await user.click(toggle);
    const group = document.getElementById("mobile-more-panel") as HTMLElement;
    expect(
      within(group).getByRole("link", { name: "Administration" }),
    ).toHaveAttribute("href", "/admin");
    await user.click(
      within(group).getByRole("link", { name: "Creator releases" }),
    );
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByRole("link", { name: "Administration" }),
    ).not.toBeInTheDocument();
  });

  it("closes on outside pointer interactions", async () => {
    const user = userEvent.setup();
    renderMobileNavigation(true);
    await user.click(screen.getByRole("button", { name: "More navigation" }));
    await user.click(document.body);
    expect(
      screen.getByRole("button", { name: "More navigation" }),
    ).toHaveAttribute("aria-expanded", "false");
  });
});
