import { expect, test } from "@playwright/test";

import { mockGateway } from "./gatewayMock";

const releaseTrackIdForTest = "00000000-0000-4000-8000-000000000502";

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

test("creator builds a release draft with its initial tracks", async ({
  page,
}) => {
  const requests = await mockGateway(page);
  await page.goto("/creator/releases");

  await page.getByLabel("Email").fill("creator@example.com");
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();

  await expect(page).toHaveURL(/\/creator\/releases$/);
  await expect(
    page.getByRole("heading", { name: "Release drafts" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "+ New release" }).click();
  await page.getByLabel("Release title").fill("After Midnight");
  await page.getByLabel("Release type").selectOption("album");
  await page.getByLabel("Release date").fill("2026-12-01");
  await page.getByLabel("Title", { exact: true }).fill("First Light");
  await page.getByRole("button", { name: "+ Add track" }).click();
  await page.getByLabel("Title", { exact: true }).nth(1).fill("Last Train");
  await page.getByRole("button", { name: "Create draft" }).click();

  await expect(page.getByRole("status")).toContainText(
    "Draft “After Midnight” was created.",
  );
  const createRequest = requests.find(
    (request) =>
      new URL(request.url()).pathname === "/v1/releases" &&
      request.method() === "POST",
  );
  expect(createRequest?.headers().authorization).toBe(
    "Bearer e2e-access-token",
  );
  expect(createRequest?.postDataJSON()).toEqual({
    creator_id: "00000000-0000-4000-8000-000000000503",
    title: "After Midnight",
    release_type: "album",
    release_date: "2026-12-01",
    tracks: [
      {
        title: "First Light",
        track_number: 1,
        disc_number: 1,
        explicit: false,
      },
      {
        title: "Last Train",
        track_number: 2,
        disc_number: 1,
        explicit: false,
      },
    ],
  });
});

test("creator grants and revokes worldwide track rights", async ({ page }) => {
  const requests = await mockGateway(page);
  await page.goto("/creator/releases");

  await page.getByLabel("Email").fill("creator@example.com");
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();

  await expect(page).toHaveURL(/\/creator\/releases$/);
  await page.getByRole("button", { name: "Manage rights" }).click();
  const streaming = page.getByRole("checkbox", { name: "Streaming" });
  const distribution = page.getByRole("checkbox", { name: "Distribution" });
  await expect(streaming).not.toBeChecked();
  await expect(distribution).not.toBeChecked();

  await distribution.click();
  await expect(distribution).toBeChecked();
  await streaming.click();
  await expect(streaming).toBeChecked();
  await streaming.click();
  await expect(streaming).not.toBeChecked();
  await expect(distribution).toBeChecked();

  const rightRequests = requests.filter((request) =>
    new URL(request.url()).pathname.endsWith(
      `/release-tracks/${releaseTrackIdForTest}/rights`,
    ),
  );
  expect(rightRequests.some((request) => request.method() === "GET")).toBe(
    true,
  );
  const storedRights = rightRequests.filter(
    (request) => request.method() === "PUT",
  );
  expect(storedRights.map((request) => request.postDataJSON())).toEqual([
    {
      territory_code: "001",
      rights_type: "distribution",
      valid_from: "2026-11-13",
      valid_until: null,
    },
    {
      territory_code: "001",
      rights_type: "streaming",
      valid_from: "2026-11-13",
      valid_until: null,
    },
  ]);
  const deleteRequest = requests.find(
    (request) =>
      request.method() === "DELETE" &&
      new URL(request.url()).pathname.endsWith(
        "/track-rights/00000000-0000-4000-8000-000000000531",
      ),
  );
  expect(deleteRequest?.headers().authorization).toBe(
    "Bearer e2e-access-token",
  );
  expect(
    rightRequests.every(
      (request) =>
        request.headers().authorization === "Bearer e2e-access-token",
    ),
  ).toBe(true);
});

test("creator creates an account and submits it for verification", async ({
  page,
}) => {
  const requests = await mockGateway(page, { emptyCreatorAccounts: true });
  await page.goto("/creator/accounts");

  await page.getByLabel("Email").fill("creator@example.com");
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();

  await expect(page).toHaveURL(/\/creator\/accounts$/);
  await expect(
    page.getByRole("heading", { name: "Creator accounts" }),
  ).toBeVisible();
  await expect(page.getByText("No creator accounts yet")).toBeVisible();
  await page.getByRole("button", { name: "+ New creator account" }).click();
  await page.getByLabel("Account type").selectOption("label");
  await page.getByLabel("Display name").fill("Northern Lights");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page.getByText("Not submitted")).toBeVisible();
  await page.getByRole("button", { name: "Submit for verification" }).click();
  await expect(page.getByText("Pending review")).toBeVisible();
  await expect(page.getByText("Awaiting moderator review")).toBeVisible();

  const createRequest = requests.find(
    (request) =>
      new URL(request.url()).pathname === "/v1/creators" &&
      request.method() === "POST",
  );
  expect(createRequest?.postDataJSON()).toEqual({
    type: "label",
    display_name: "Northern Lights",
  });
  const verificationRequest = requests.find((request) =>
    new URL(request.url()).pathname.endsWith("/verification-submissions"),
  );
  expect(verificationRequest?.method()).toBe("POST");
  expect(verificationRequest?.headers().authorization).toBe(
    "Bearer e2e-access-token",
  );
});

test("administrator can open the protected administration workspace", async ({
  page,
}) => {
  await mockGateway(page, { roles: ["listener", "admin"] });
  await page.goto("/admin");

  await page.getByLabel("Email").fill("admin@example.com");
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();

  await expect(page).toHaveURL(/\/admin$/);
  await expect(
    page.getByRole("heading", { name: "Administration", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("listener@example.com")).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Administration" }),
  ).toBeVisible();
});
