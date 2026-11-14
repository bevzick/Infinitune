import { expect, type Page, test } from "@playwright/test";

import { createPublishingJourneyMock } from "./creatorPublishingGateway";

const png = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9B0hV7wAAAAASUVORK5CYII=",
  "base64",
);
const wav = Buffer.from("RIFF0000WAVEfmt 0000000000000000data0000", "ascii");

async function signIn(page: Page, path: string, email: string) {
  await page.goto(path);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(
    path === "/login" ? /\/$/ : new RegExp(`${path.replaceAll("/", "\\/")}$`),
  );
  // Simulate a refresh cookie so the independent sessions survive reload.
  await page.context().addCookies([
    {
      name: "infinitune_csrf",
      value: "e2e-csrf",
      url: "http://127.0.0.1:5173",
    },
  ]);
}

test("creator and independent moderator publish a licensed release visible to a listener", async ({
  page,
  browser,
}) => {
  test.setTimeout(90_000);
  const gateway = createPublishingJourneyMock();
  const moderator = await browser.newPage();
  const listener = await browser.newPage();
  try {
    await gateway.attach(page, "creator");
    await gateway.attach(moderator, "moderator");
    await gateway.attach(listener, "listener");

    // Account creation and independent verification.
    await signIn(page, "/creator/accounts", "creator@example.com");
    await page.getByRole("button", { name: "+ New creator account" }).click();
    await page.getByLabel("Account type").selectOption("artist");
    await page.getByLabel("Display name").fill("Northern Lights");
    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page.getByText("Not submitted")).toBeVisible();
    await page.getByRole("button", { name: "Submit for verification" }).click();
    await expect(page.getByText("Pending review")).toBeVisible();

    await signIn(moderator, "/admin", "moderator@example.com");
    await moderator.getByRole("button", { name: /Content review/ }).click();
    await expect(moderator.getByText("Northern Lights")).toBeVisible();
    await moderator.getByRole("button", { name: "Verify" }).click();
    await expect(
      moderator.getByText("No pending creators found."),
    ).toBeVisible();

    // Create one track and prove readiness is blocked until all prerequisites exist.
    await page.goto("/creator/releases");
    await page.getByRole("button", { name: "+ New release" }).click();
    await page.getByLabel("Release title").fill("After Midnight");
    await page.getByLabel("Release type").selectOption("single");
    await page.getByLabel("Release date").fill("2026-12-01");
    await page.getByLabel("Title", { exact: true }).fill("Night Signal");
    await page.getByRole("button", { name: "Create draft" }).click();
    const release = page
      .locator(".creator-release-card")
      .filter({ has: page.getByRole("heading", { name: "After Midnight" }) });
    await expect(release).toBeVisible();
    await release.getByRole("button", { name: "Manage lifecycle" }).click();
    await expect(
      release.getByRole("region", { name: "Submission readiness" }),
    ).toContainText("cover missing");
    await expect(
      release.getByRole("button", { name: "Submit for review" }),
    ).toBeDisabled();
    await release.getByRole("button", { name: "Close lifecycle" }).click();

    await release.getByRole("button", { name: "Manage cover" }).click();
    await release
      .getByLabel("Cover image")
      .setInputFiles({ name: "cover.png", mimeType: "image/png", buffer: png });
    await release.getByRole("button", { name: "Upload cover" }).click();
    await expect(release.getByRole("status")).toContainText(
      "Cover uploaded successfully.",
    );
    await expect(
      release.getByRole("img", { name: "Cover stored" }),
    ).toBeVisible();
    await release.getByRole("button", { name: "Close cover" }).click();

    await release.getByRole("button", { name: "Upload audio" }).click();
    await release.getByLabel("Audio file for Night Signal").setInputFiles({
      name: "signal.wav",
      mimeType: "audio/wav",
      buffer: wav,
    });
    await release.getByRole("button", { name: "Upload Night Signal" }).click();
    await expect(release.getByRole("status")).toContainText(
      "Audio processing has been queued.",
    );
    await release.getByRole("button", { name: "Close uploads" }).click();

    await release.getByRole("button", { name: "Manage rights" }).click();
    await release.getByRole("checkbox", { name: "Streaming" }).click();
    await expect(
      release.getByRole("checkbox", { name: "Streaming" }),
    ).toBeChecked();
    await release.getByRole("checkbox", { name: "Distribution" }).click();
    await expect(
      release.getByRole("checkbox", { name: "Distribution" }),
    ).toBeChecked();
    await release.getByRole("button", { name: "Close rights" }).click();

    await release.getByRole("button", { name: "Manage lifecycle" }).click();
    await expect(
      release.getByRole("region", { name: "Submission readiness" }),
    ).toContainText("media not ready");
    await expect(
      release.getByRole("button", { name: "Submit for review" }),
    ).toBeDisabled();
    gateway.completeAudioProcessing();
    await release.getByRole("button", { name: "Refresh status" }).click();
    await expect(release.getByRole("status")).toContainText(
      "All submission requirements are satisfied.",
    );
    await release.getByRole("button", { name: "Submit for review" }).click();
    await expect(
      release.getByRole("region", { name: "Release lifecycle" }),
    ).toContainText("Current status: pending review");

    // The same creator must not be able to approve their own submission.
    const forbidden = await page.evaluate(async (id: string) => {
      const response = await fetch(
        `http://localhost:8000/v1/releases/${id}/moderation-reviews`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: "Bearer e2e-creator-token",
          },
          body: JSON.stringify({ decision: "approved" }),
        },
      );
      return { status: response.status, payload: await response.json() };
    }, gateway.ids.release);
    expect(forbidden).toMatchObject({
      status: 403,
      payload: { error: { code: "self_approval_forbidden" } },
    });

    await moderator.reload();
    await moderator.getByRole("button", { name: /Content review/ }).click();
    await expect(
      moderator.getByText("After Midnight", { exact: true }),
    ).toBeVisible();
    await moderator.getByRole("button", { name: "Approve" }).click();
    await expect(
      moderator.getByText("No pending review releases found."),
    ).toBeVisible();

    // Catalog remains empty until asynchronous publication delivery.
    await signIn(listener, "/library", "listener@example.com");
    await listener.goto("/search");
    await expect(
      listener.getByRole("heading", { name: "Search music" }),
    ).toBeVisible();
    await expect(
      listener.getByText("No tracks match these filters"),
    ).toBeVisible();
    await expect(
      listener.getByRole("link", { name: "Night Signal" }),
    ).toHaveCount(0);

    await page.reload();
    await release.getByRole("button", { name: "Manage lifecycle" }).click();
    await expect(
      release.getByRole("region", { name: "Release lifecycle" }),
    ).toContainText("Current status: approved");
    await release.getByRole("button", { name: "Publish release" }).click();
    await expect(
      release.getByRole("region", { name: "Release lifecycle" }),
    ).toContainText("Current status: published");
    await listener.reload();
    await expect(
      listener.getByText("No tracks match these filters"),
    ).toBeVisible();
    await expect(
      listener.getByRole("link", { name: "Night Signal" }),
    ).toHaveCount(0);

    gateway.syncCatalog();
    await listener.reload();
    await expect(
      listener.getByRole("link", { name: "Night Signal" }),
    ).toBeVisible();
    const playbackResponsePromise = listener.waitForResponse(
      (response) =>
        response.request().method() === "GET" &&
        new URL(response.url()).pathname ===
          `/v1/media/tracks/${gateway.ids.catalogTrack}/playback`,
    );
    await listener.getByRole("button", { name: "Play Night Signal" }).click();
    expect((await playbackResponsePromise).status()).toBe(200);
    await expect(
      listener.getByRole("region", { name: "Audio player" }),
    ).toContainText("Night Signal");

    const orderedPaths = gateway.requests
      .filter(
        ({ request }) =>
          request.method() !== "GET" || request.url().includes("/playback"),
      )
      .map(
        ({ actor, request }) =>
          `${actor}:${request.method()}:${new URL(request.url()).pathname}`,
      );
    const expected = [
      `creator:POST:/v1/creators`,
      `creator:POST:/v1/creators/${gateway.ids.account}/verification-submissions`,
      `moderator:POST:/v1/creators/${gateway.ids.account}/verification-reviews`,
      "creator:POST:/v1/releases",
      `creator:POST:/v1/releases/${gateway.ids.release}/cover-uploads`,
      `creator:PUT:/v1/media/cover-uploads/${gateway.ids.coverUpload}`,
      `creator:POST:/v1/release-tracks/${gateway.ids.track}/uploads`,
      `creator:PUT:/v1/media/uploads/${gateway.ids.audioUpload}`,
      `creator:POST:/v1/releases/${gateway.ids.release}/submissions`,
      `moderator:POST:/v1/releases/${gateway.ids.release}/moderation-reviews`,
      `creator:POST:/v1/releases/${gateway.ids.release}/publications`,
      `listener:GET:/v1/media/tracks/${gateway.ids.catalogTrack}/playback`,
    ];
    let cursor = -1;
    for (const step of expected) {
      cursor = orderedPaths.indexOf(step, cursor + 1);
      expect(
        cursor,
        `${step} must follow the previous publishing step`,
      ).toBeGreaterThan(-1);
    }
    const binaryRequests = gateway.requests.filter(
      ({ request }) =>
        request.method() === "PUT" &&
        /\/v1\/media\/(cover-uploads|uploads)\//.test(request.url()),
    );
    expect(binaryRequests).toHaveLength(2);
    for (const { actor, request } of binaryRequests) {
      expect(actor).toBe("creator");
      expect(request.headers().authorization).toBe("Bearer e2e-creator-token");
      const isCover = request.url().includes("cover-uploads");
      expect(request.headers()["content-type"]).toBe(
        isCover ? "image/png" : "audio/wav",
      );
      expect(request.postDataBuffer()).toEqual(isCover ? png : wav);
    }
    const approval = gateway.requests.find(
      ({ actor, request }) =>
        actor === "moderator" &&
        request.method() === "POST" &&
        request.url().endsWith("/moderation-reviews"),
    );
    expect(approval?.request.headers().authorization).toBe(
      "Bearer e2e-moderator-token",
    );
    const playback = gateway.requests.find(
      ({ actor, request }) =>
        actor === "listener" &&
        request.method() === "GET" &&
        request.url().includes("/playback"),
    );
    expect(playback?.request.headers().authorization).toBe(
      "Bearer e2e-listener-token",
    );
  } finally {
    await moderator.close();
    await listener.close();
  }
});
