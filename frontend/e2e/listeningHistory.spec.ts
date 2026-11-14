import { expect, type Page, test } from "@playwright/test";

import {
  createListeningHistoryGateway,
  type HistoryActor,
  historyOtherTrackId,
  historyTrackId,
} from "./listeningHistoryGateway";

const appURL = "http://127.0.0.1:5173";
const gatewayURL = "http://localhost:8000";

async function signIn(
  page: Page,
  actor: HistoryActor,
  destination = "/history",
) {
  await page.goto("/history");
  await expect(
    page.getByRole("heading", { name: "Welcome back" }),
  ).toBeVisible();
  await page.getByLabel("Email").fill(`${actor}@example.com`);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(`${appURL}/history`);
  await page
    .context()
    .addCookies([
      { name: "infinitune_csrf", value: "history-e2e-csrf", url: appURL },
    ]);
  if (destination !== "/history") {
    await page.goto(destination);
    await expect(page).toHaveURL(`${appURL}${destination}`);
  }
}

const eventsList = (page: Page) =>
  page
    .getByRole("region", { name: "Listening events" })
    .locator("li.history-event");

test("completed playback records one idempotent event visible only to its owner", async ({
  browser,
}) => {
  test.setTimeout(60_000);
  const gateway = createListeningHistoryGateway();
  const aliceContext = await browser.newContext({ baseURL: appURL });
  const bobContext = await browser.newContext({ baseURL: appURL });
  try {
    const alice = await aliceContext.newPage();
    const bob = await bobContext.newPage();
    await gateway.attach(alice, "alice");
    await gateway.attach(bob, "bob");

    // This private route requires authentication even if the user knows its URL.
    await alice.goto("/history");
    await expect(
      alice.getByRole("heading", { name: "Welcome back" }),
    ).toBeVisible();
    await expect(alice).toHaveURL(/\/login$/);

    await signIn(alice, "alice", `/tracks/${historyTrackId}`);
    await expect(
      alice.getByRole("button", { name: "Play track" }),
    ).toBeVisible();
    await alice.getByRole("button", { name: "Play track" }).click();
    await expect(
      alice.getByRole("region", { name: "Audio player" }),
    ).toContainText("Night Drive");

    // The browser plays a two-second WAV; PlayerProvider reports on audio 'ended'.
    await expect
      .poll(() => gateway.eventsFor("alice").length, {
        timeout: 15_000,
        message: "player should report one qualifying completed listen",
      })
      .toBe(1);
    const saved = gateway.eventsFor("alice")[0];
    expect(saved.track_id).toBe(historyTrackId);
    expect(saved.context_type).toBe("direct");
    expect(saved.device_type).toBe("web");
    expect(saved.listened_ms).toBeGreaterThanOrEqual(1_000);
    expect(saved.completed).toBe(true);

    const reported = gateway.requests.find(
      ({ actor, request }) =>
        actor === "alice" &&
        request.method() === "POST" &&
        new URL(request.url()).pathname === "/v1/history/events",
    );
    expect(reported?.request.headers().authorization).toBe(
      "Bearer history-alice-token",
    );
    expect(reported?.request.postDataJSON()).toMatchObject({
      event_id: saved.event_id,
      track_id: historyTrackId,
      context_type: "direct",
      device_type: "web",
    });

    // The same event ID/payload is idempotent; no second history row appears.
    const replay = await alice.evaluate(
      async ({ url, event, token }) => {
        const response = await fetch(`${url}/v1/history/events`, {
          method: "POST",
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify(event),
        });
        return { status: response.status, body: await response.json() };
      },
      {
        url: gatewayURL,
        token: "history-alice-token",
        event: reported?.request.postDataJSON(),
      },
    );
    expect(replay.status).toBe(200);
    expect(replay.body).toMatchObject({
      event_id: saved.event_id,
      recorded: true,
    });
    expect(gateway.eventsFor("alice")).toHaveLength(1);

    await alice.goto("/history");
    await expect(
      alice.getByRole("heading", { name: "Listening history", exact: true }),
    ).toBeVisible();
    await expect(eventsList(alice)).toHaveCount(1);
    await expect(
      eventsList(alice).getByRole("link", { name: "Night Drive" }),
    ).toBeVisible();
    await alice.reload();
    await expect(eventsList(alice)).toHaveCount(1);

    await signIn(bob, "bob");
    await expect(
      bob.getByText("No listening events match these filters."),
    ).toBeVisible();
    expect(gateway.eventsFor("bob")).toHaveLength(0);
    const accessViolation = await bob.evaluate(async (url) => {
      const response = await fetch(`${url}/v1/history/events`, {
        headers: { Authorization: "Bearer history-alice-token" },
      });
      return response.status;
    }, gatewayURL);
    expect(accessViolation).toBe(401);
    expect(
      gateway.requests
        .filter(
          ({ actor, request }) =>
            actor === "bob" &&
            request.method() === "GET" &&
            new URL(request.url()).pathname === "/v1/history/events",
        )
        .some(
          ({ request }) =>
            request.headers().authorization === "Bearer history-bob-token",
        ),
    ).toBe(true);
  } finally {
    await Promise.all([aliceContext.close(), bobContext.close()]);
  }
});

test("private history paginates, filters and clears without affecting another user", async ({
  browser,
}) => {
  const gateway = createListeningHistoryGateway();
  gateway.seed("alice", 22);
  gateway.seed("alice", 1, "2026-11-12T10:00:00Z");
  gateway.seed("bob", 1);
  const aliceContext = await browser.newContext({ baseURL: appURL });
  try {
    const alice = await aliceContext.newPage();
    await gateway.attach(alice, "alice");
    await signIn(alice, "alice");
    await expect(eventsList(alice)).toHaveCount(20);
    await expect(
      alice.getByRole("button", { name: "Load more" }),
    ).toBeVisible();
    await alice.getByRole("button", { name: "Load more" }).click();
    await expect(eventsList(alice)).toHaveCount(23);
    const shown = await eventsList(alice).count();
    expect(shown).toBe(23);
    const cursorRequest = gateway.requests.find(
      ({ actor, request }) =>
        actor === "alice" &&
        request.method() === "GET" &&
        new URL(request.url()).searchParams.has("cursor"),
    );
    expect(cursorRequest).toBeDefined();
    expect(
      new URL(cursorRequest?.request.url() || gatewayURL).searchParams.get(
        "limit",
      ),
    ).toBe("20");

    const filters = alice.getByRole("form", { name: "History filters" });
    await filters.getByLabel("Playback").selectOption("false");
    await filters.getByRole("button", { name: "Apply filters" }).click();
    await expect(eventsList(alice)).toHaveCount(11);
    await expect(alice.getByText("Partial").first()).toBeVisible();
    await filters.getByLabel("Track ID").fill(historyOtherTrackId);
    await filters.getByLabel("Context").selectOption("search");
    await filters.getByRole("button", { name: "Apply filters" }).click();
    await expect(eventsList(alice)).toHaveCount(11);
    const filteredRequest = [...gateway.requests]
      .reverse()
      .find(
        ({ actor, request }) =>
          actor === "alice" &&
          request.method() === "GET" &&
          new URL(request.url()).pathname === "/v1/history/events" &&
          new URL(request.url()).searchParams.get("track_id") ===
            historyOtherTrackId,
      );
    expect(filteredRequest).toBeDefined();
    expect(
      new URL(filteredRequest?.request.url() || gatewayURL).searchParams.get(
        "context_type",
      ),
    ).toBe("search");

    await filters.getByRole("button", { name: "Reset" }).click();
    await expect(eventsList(alice)).toHaveCount(20);

    await filters.getByLabel("From").fill("2026-11-12");
    await filters.getByRole("button", { name: "Apply filters" }).click();
    await expect(eventsList(alice)).toHaveCount(1);
    const dateRequest = [...gateway.requests]
      .reverse()
      .find(
        ({ actor, request }) =>
          actor === "alice" &&
          request.method() === "GET" &&
          new URL(request.url()).searchParams.has("from"),
      );
    expect(
      new URL(dateRequest?.request.url() || gatewayURL).searchParams.get(
        "from",
      ),
    ).toBe(
      await alice.evaluate(() => new Date("2026-11-12T00:00:00").toISOString()),
    );
    await filters.getByRole("button", { name: "Reset" }).click();
    await expect(eventsList(alice)).toHaveCount(20);

    const beforeDate = alice.getByLabel(/Delete events before date/);
    await beforeDate.fill("2026-11-11");
    alice.once("dialog", (dialog) => dialog.dismiss());
    await alice.getByRole("button", { name: "Clear history" }).click();
    expect(gateway.eventsFor("alice")).toHaveLength(23);

    alice.once("dialog", (dialog) => dialog.accept());
    await alice.getByRole("button", { name: "Clear history" }).click();
    await expect(
      alice.locator(".history-clear").getByRole("status"),
    ).toContainText("22 listening event(s) deleted.");
    await expect(eventsList(alice)).toHaveCount(1);
    expect(gateway.eventsFor("alice")).toHaveLength(1);
    const deleteRequest = gateway.requests.find(
      ({ actor, request }) =>
        actor === "alice" &&
        request.method() === "DELETE" &&
        new URL(request.url()).pathname === "/v1/history/events",
    );
    expect(deleteRequest).toBeDefined();
    expect(
      new URL(deleteRequest?.request.url() || gatewayURL).searchParams.get(
        "before",
      ),
    ).toBe(
      await alice.evaluate(() => new Date("2026-11-11T00:00:00").toISOString()),
    );
    expect(gateway.eventsFor("bob")).toHaveLength(1);

    await beforeDate.fill("");
    alice.once("dialog", (dialog) => dialog.accept());
    await alice.getByRole("button", { name: "Clear history" }).click();
    await expect(
      alice.getByText("No listening events match these filters."),
    ).toBeVisible();
    expect(gateway.eventsFor("alice")).toHaveLength(0);
    expect(gateway.eventsFor("bob")).toHaveLength(1);
  } finally {
    await aliceContext.close();
  }
});

test("history load error offers a retry without exposing other users' events", async ({
  page,
}) => {
  const gateway = createListeningHistoryGateway();
  gateway.seed("bob", 1);
  gateway.setListUnavailable("alice", true);
  await gateway.attach(page, "alice");
  await signIn(page, "alice");
  await expect(page.getByRole("alert")).toContainText("history_unavailable");
  gateway.setListUnavailable("alice", false);
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(
    page.getByText("No listening events match these filters."),
  ).toBeVisible();
  expect(gateway.eventsFor("bob")).toHaveLength(1);
});
