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

test("creator discovers and imports licensed provider audio", async ({
  page,
}) => {
  const requests = await mockGateway(page);
  await page.goto("/creator/import");

  await expect(
    page.getByRole("heading", { name: "Welcome back" }),
  ).toBeVisible();
  await page.getByLabel("Email").fill("creator@example.com");
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();

  await expect(page).toHaveURL(/\/creator\/import$/);
  await expect(
    page.getByRole("heading", {
      name: "Import music from a licensed source.",
    }),
  ).toBeVisible();
  await expect(page.getByLabel("Draft release track")).toHaveValue(
    "00000000-0000-4000-8000-000000000502",
  );

  await page.getByLabel("Track name").fill("night signal");
  await page.getByRole("button", { name: "Search source" }).click();
  await expect(page.getByText("CC BY 4.0")).toBeVisible();
  await page.getByRole("button", { name: "Import track" }).click();

  await expect(page.getByRole("status")).toContainText(
    "is being imported for “Night Signal”",
  );
  const importRequest = requests.find(
    (request) => new URL(request.url()).pathname === "/v1/media/imports",
  );
  expect(importRequest?.method()).toBe("POST");
  expect(importRequest?.headers().authorization).toBe(
    "Bearer e2e-access-token",
  );
  expect(importRequest?.postDataJSON()).toEqual({
    content_track_id: "00000000-0000-4000-8000-000000000502",
    provider: "jamendo",
    external_id: "42",
  });
});
