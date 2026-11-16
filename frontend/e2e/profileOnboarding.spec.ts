import { expect, test } from "@playwright/test";

import { mockGateway } from "./gatewayMock";

test("new listener completes profile onboarding before entering the application", async ({
  page,
}) => {
  const requests = await mockGateway(page, { missingProfile: true });
  const password = "correct-horse-battery-staple";

  await page.goto("/register");
  await page.getByLabel("Email").fill("new-listener@example.com");
  await page.getByLabel(/^Password/).fill(password);
  await page.getByLabel("Confirm password").fill(password);
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole("status")).toContainText("Account created");
  await expect(page.getByLabel("Email")).toHaveValue(
    "new-listener@example.com",
  );
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();

  await expect(page).toHaveURL(/\/profile$/);
  await expect(
    page.getByRole("heading", { name: "Create your listener profile." }),
  ).toBeVisible();
  await page.getByRole("textbox", { name: /^Username/ }).fill("new_listener");
  await page.getByLabel("Display name").fill("New Listener");
  await page.getByRole("button", { name: "Create profile" }).click();

  await expect(page).toHaveURL(/\/$/);
  await expect(
    page.getByRole("heading", { name: /Find your new sound/i }),
  ).toBeVisible();

  const registration = requests.find(
    (request) =>
      request.method() === "POST" &&
      new URL(request.url()).pathname === "/v1/auth/register",
  );
  expect(registration?.postDataJSON()).toEqual({
    email: "new-listener@example.com",
    password,
  });

  const profileChecks = requests.filter(
    (request) =>
      request.method() === "GET" &&
      new URL(request.url()).pathname === "/v1/users/me/profile",
  );
  expect(profileChecks.length).toBeGreaterThanOrEqual(2);
  const profileCreation = requests.find(
    (request) =>
      request.method() === "POST" &&
      new URL(request.url()).pathname === "/v1/users/me/profile",
  );
  expect(profileCreation?.headers().authorization).toBe(
    "Bearer e2e-access-token",
  );
  expect(profileCreation?.postDataJSON()).toEqual({
    username: "new_listener",
    display_name: "New Listener",
  });
});
