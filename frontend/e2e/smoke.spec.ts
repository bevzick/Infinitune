import { expect, test } from "@playwright/test";

import { mockGateway } from "./gatewayMock";

test("listener can discover and search the public catalog", async ({
  page,
}) => {
  await mockGateway(page);
  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: /Find your new sound/i }),
  ).toBeVisible();
  await expect(page.getByText("Night City").first()).toBeVisible();

  await page.getByRole("link", { name: "Search" }).first().click();
  await expect(page).toHaveURL(/\/search$/);
  await page.getByRole("searchbox", { name: "Track title" }).fill("night");
  await page.getByRole("button", { name: "Search" }).click();

  await expect(page).toHaveURL(/\/search\?q=night$/);
  await expect(page.getByText("Night Drive").first()).toBeVisible();
  await expect(page.getByText("Best match")).toBeVisible();
});

test("protected library redirects to login and resumes after authentication", async ({
  page,
}) => {
  const requests = await mockGateway(page);
  await page.goto("/library");

  await expect(
    page.getByRole("heading", { name: "Welcome back" }),
  ).toBeVisible();
  await page.getByLabel("Email").fill("listener@example.com");
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();

  await expect(page).toHaveURL(/\/library$/);
  await expect(page.getByRole("heading", { name: "Library" })).toBeVisible();
  await expect(page.getByRole("link", { name: /Night Focus/ })).toBeVisible();
  expect(
    requests.some(
      (request) =>
        new URL(request.url()).pathname === "/v1/playlists" &&
        request.headers().authorization === "Bearer e2e-access-token",
    ),
  ).toBe(true);
});
