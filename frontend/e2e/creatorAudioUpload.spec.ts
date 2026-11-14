import { expect, test } from "@playwright/test";

import { mockGateway } from "./gatewayMock";

const trackId = "00000000-0000-4000-8000-000000000502";
const uploadId = "00000000-0000-4000-8000-000000000551";
const audioBytes = Buffer.from(
  "RIFF0000WAVEfmt 0000000000000000data0000",
  "ascii",
);

async function openCreatorUploads(page: import("@playwright/test").Page) {
  await page.goto("/creator/releases");
  await page.getByLabel("Email").fill("creator@example.com");
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/creator\/releases$/);
  await page.getByRole("button", { name: "Upload audio" }).click();
  await expect(
    page.getByRole("region", { name: "Track audio uploads" }),
  ).toBeVisible();
}

test("creator uploads raw audio through a session and sees queued processing", async ({
  page,
}) => {
  const requests = await mockGateway(page);
  await openCreatorUploads(page);

  await page.getByLabel("Audio file for Night Signal").setInputFiles({
    name: "night-signal.wav",
    mimeType: "audio/wav",
    buffer: audioBytes,
  });
  await page.getByRole("button", { name: "Upload Night Signal" }).click();

  await expect(page.getByRole("status")).toContainText(
    "Uploaded. Audio processing has been queued.",
  );

  const sessionRequest = requests.find(
    (request) =>
      request.method() === "POST" &&
      new URL(request.url()).pathname ===
        `/v1/release-tracks/${trackId}/uploads`,
  );
  expect(sessionRequest).toBeDefined();
  expect(sessionRequest?.headers().authorization).toBe(
    "Bearer e2e-access-token",
  );
  expect(sessionRequest?.headers()["content-type"]).toContain(
    "application/json",
  );
  const sessionBody = sessionRequest?.postDataJSON() as {
    idempotency_key: string;
    original_name: string;
    mime_type: string;
    size_bytes: number;
  };
  expect(sessionBody.idempotency_key).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
  );
  expect(sessionBody).toEqual({
    idempotency_key: sessionBody.idempotency_key,
    original_name: "night-signal.wav",
    mime_type: "audio/wav",
    size_bytes: audioBytes.length,
  });

  const uploadRequests = requests.filter(
    (request) =>
      request.method() === "PUT" &&
      new URL(request.url()).pathname === `/v1/media/uploads/${uploadId}`,
  );
  expect(uploadRequests).toHaveLength(1);
  const uploadRequest = uploadRequests[0];
  expect(uploadRequest.headers().authorization).toBe("Bearer e2e-access-token");
  expect(uploadRequest.headers()["content-type"]).toBe("audio/wav");
  expect(uploadRequest.postDataBuffer()).toEqual(audioBytes);
});

test("creator sees a server upload error without a false success state", async ({
  page,
}) => {
  const requests = await mockGateway(page, { audioUploadFailure: true });
  await openCreatorUploads(page);

  await page.getByLabel("Audio file for Night Signal").setInputFiles({
    name: "night-signal.wav",
    mimeType: "audio/wav",
    buffer: audioBytes,
  });

  const sessionResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname ===
        `/v1/release-tracks/${trackId}/uploads`,
  );
  const uploadResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "PUT" &&
      new URL(response.url()).pathname === `/v1/media/uploads/${uploadId}`,
  );

  await page.getByRole("button", { name: "Upload Night Signal" }).click();

  expect((await sessionResponsePromise).status()).toBe(201);
  const uploadResponse = await uploadResponsePromise;
  expect(uploadResponse.status()).toBe(503);
  await uploadResponse.finished();

  await expect(page.getByRole("alert")).toContainText("ingest_unavailable");
  await expect(
    page.getByText("Uploaded. Audio processing has been queued."),
  ).toHaveCount(0);

  const failedUploadRequests = requests.filter(
    (request) =>
      request.method() === "PUT" &&
      new URL(request.url()).pathname === `/v1/media/uploads/${uploadId}`,
  );
  expect(failedUploadRequests).toHaveLength(1);
  expect(failedUploadRequests[0].headers().authorization).toBe(
    "Bearer e2e-access-token",
  );
  expect(failedUploadRequests[0].postDataBuffer()).toEqual(audioBytes);
});

test("creator cannot upload an unsupported audio file", async ({ page }) => {
  const requests = await mockGateway(page);
  await openCreatorUploads(page);

  await page.getByLabel("Audio file for Night Signal").setInputFiles({
    name: "notes.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("not audio"),
  });

  await expect(page.getByRole("alert")).toContainText(
    "Choose a WAV, FLAC, MP3 or M4A file.",
  );
  await expect(
    page.getByRole("button", { name: "Upload Night Signal" }),
  ).toBeDisabled();
  expect(
    requests.filter((request) =>
      new URL(request.url()).pathname.endsWith(
        `/release-tracks/${trackId}/uploads`,
      ),
    ),
  ).toHaveLength(0);
});
