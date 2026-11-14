import { expect, test } from "@playwright/test";

import { mockGateway } from "./gatewayMock";

test("mobile guests can open sign-in and browse compact navigation", async ({
  page,
}) => {
  await page.setViewportSize({ width: 320, height: 700 });
  await mockGateway(page);
  await page.goto("/");

  const nav = page.getByRole("navigation", { name: "Mobile navigation" });
  await expect(nav).toBeVisible();
  await expect(nav.locator(":scope > a")).toHaveCount(4);
  await expect(
    page.getByRole("banner").getByRole("link", { name: "Sign in" }),
  ).toBeVisible();

  const more = nav.getByRole("button", { name: "More navigation" });
  await more.click();
  await expect(more).toHaveAttribute("aria-expanded", "true");
  await expect(nav.getByRole("link", { name: "Artists" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(more).toBeFocused();
  await expect(more).toHaveAttribute("aria-expanded", "false");

  await more.click();
  await nav.getByRole("link", { name: "Artists" }).click();
  await expect(page).toHaveURL(/\/artists$/);
  await expect(more).toHaveAttribute("aria-expanded", "false");
});

test("mobile creator links remain reachable and connection status recovers", async ({
  page,
}) => {
  await page.setViewportSize({ width: 375, height: 812 });
  await mockGateway(page);
  await page.goto("/creator/releases");
  await page.getByLabel("Email").fill("creator@example.com");
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/creator\/releases$/);

  const nav = page.getByRole("navigation", { name: "Mobile navigation" });
  const more = nav.getByRole("button", { name: "More navigation" });
  await more.click();
  await expect(
    nav.getByRole("link", { name: "Creator accounts" }),
  ).toBeVisible();
  await nav.getByRole("link", { name: "Creator accounts" }).click();
  await expect(page).toHaveURL(/\/creator\/accounts$/);

  await page.evaluate(() => window.dispatchEvent(new Event("offline")));
  await expect(
    page.getByRole("status").filter({ hasText: "You are offline" }),
  ).toBeVisible();
  await page.evaluate(() => window.dispatchEvent(new Event("online")));
  await expect(page.getByText("You are offline", { exact: false })).toHaveCount(
    0,
  );
  await expect(
    page.getByRole("banner").getByRole("button", { name: "Sign out" }),
  ).toBeVisible();
});
