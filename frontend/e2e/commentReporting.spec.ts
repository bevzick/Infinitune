import { expect, type Page, test } from "@playwright/test";

import {
  communityIds,
  createCommunityJourneyMock,
} from "./commentReportingGateway";

const trackPath = `/tracks/${communityIds.track}`;
const gatewayURL = "http://localhost:8000";
const appURL = "http://127.0.0.1:5173";

async function signIn(page: Page, actor: "author" | "reporter" | "moderator") {
  await page.goto("/login");
  await page.getByLabel("Email").fill(`${actor}@example.com`);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(`${appURL}/`);
  await page
    .context()
    .addCookies([{ name: "infinitune_csrf", value: "e2e-csrf", url: appURL }]);
}

const ownComment = (page: Page, text: string) =>
  page
    .getByRole("article", { name: "Comment by you" })
    .filter({ hasText: text });

// Browser-only integration: shared mock for distinct author, reporter and moderator.
// A separate real-service system smoke is required for actual persistence and permissions.
test("comment, reply, edit, delete, like, report and moderator hide are reflected to listeners", async ({
  browser,
}) => {
  test.setTimeout(90_000);
  const gateway = createCommunityJourneyMock();
  const anonymousContext = await browser.newContext({ baseURL: appURL });
  const authorContext = await browser.newContext({ baseURL: appURL });
  const reporterContext = await browser.newContext({ baseURL: appURL });
  const moderatorContext = await browser.newContext({ baseURL: appURL });
  try {
    const anonymous = await anonymousContext.newPage();
    const author = await authorContext.newPage();
    const reporter = await reporterContext.newPage();
    const moderator = await moderatorContext.newPage();
    await gateway.attach(anonymous, "reporter");
    await gateway.attach(author, "author");
    await gateway.attach(reporter, "reporter");
    await gateway.attach(moderator, "moderator");

    // Public comments remain readable without a token.
    await anonymous.goto(trackPath);
    await expect(
      anonymous.getByRole("region", { name: "Track comments" }),
    ).toBeVisible();
    await expect(anonymous.getByText("No comments yet.")).toBeVisible();
    await expect(
      anonymous
        .getByRole("region", { name: "Track comments" })
        .getByRole("link", { name: "Sign in" }),
    ).toBeVisible();

    await signIn(author, "author");
    await author.goto(trackPath);
    await expect(author.getByLabel("Write a comment")).toBeVisible();
    await author
      .getByLabel("Write a comment")
      .fill("An original listening note");
    await author.getByRole("button", { name: "Post comment" }).click();
    await expect(
      ownComment(author, "An original listening note"),
    ).toBeVisible();

    // A comment owner can edit their own message.
    await ownComment(author, "An original listening note")
      .getByRole("button", { name: "Edit" })
      .click();
    await author.getByLabel("Edit your comment").fill("Updated listening note");
    await author.getByRole("button", { name: "Save edit" }).click();
    await expect(ownComment(author, "Updated listening note")).toBeVisible();
    await expect(author.getByText("An original listening note")).toHaveCount(0);

    // Reply using a real parent comment ID, then delete with confirmation.
    await ownComment(author, "Updated listening note")
      .getByRole("button", { name: "Reply" })
      .click();
    await author.getByLabel(/^Reply to/).fill("A short follow-up reply");
    await author.getByRole("button", { name: "Post reply" }).click();
    await expect(ownComment(author, "A short follow-up reply")).toBeVisible();
    author.once("dialog", (dialog) => dialog.accept());
    await ownComment(author, "A short follow-up reply")
      .getByRole("button", { name: "Delete" })
      .click();
    await expect(author.getByText("A short follow-up reply")).toHaveCount(0);

    // A second identity sees the latest comment, can like and report, but cannot edit it.
    await signIn(reporter, "reporter");
    await reporter.goto(trackPath);
    const original = reporter
      .getByRole("article", { name: /^Comment by/ })
      .filter({ hasText: "Updated listening note" });
    await expect(original).toBeVisible();
    await expect(original.getByRole("button", { name: "Edit" })).toHaveCount(0);
    await expect(
      original.getByRole("button", { name: "Like comment" }),
    ).toBeEnabled();
    await original.getByRole("button", { name: "Like comment" }).click();
    await expect(
      original.getByRole("button", { name: "Unlike comment" }),
    ).toHaveAttribute("aria-pressed", "true");
    await original.getByRole("button", { name: "Report", exact: true }).click();
    await original.getByLabel("Report reason").selectOption("spam");
    await original
      .getByLabel("Report details")
      .fill("This comment needs review");
    await original.getByRole("button", { name: "Send report" }).click();
    await expect(
      original.getByRole("button", { name: "Reported" }),
    ).toBeDisabled();

    expect(gateway.reports).toHaveLength(1);
    const reportId = gateway.reports[0].id;
    const reportRequest = gateway.requests.find(
      ({ actor, request }) =>
        actor === "reporter" &&
        request.method() === "POST" &&
        new URL(request.url()).pathname.endsWith("/reports"),
    );
    expect(reportRequest?.request.headers().authorization).toBe(
      "Bearer community-reporter-token",
    );
    expect(reportRequest?.request.postDataJSON()).toEqual({
      reason: "spam",
      description: "This comment needs review",
    });

    await signIn(moderator, "moderator");
    await moderator.goto("/admin");
    await expect(
      moderator.getByRole("heading", { name: "Administration", exact: true }),
    ).toBeVisible();
    await moderator.getByRole("button", { name: /Comment reports/ }).click();
    await expect(moderator.getByText("Updated listening note")).toBeVisible();
    await moderator
      .getByLabel(`Moderation note for report ${reportId}`)
      .fill("Verified by moderator");
    await moderator.getByRole("button", { name: "Hide comment" }).click();
    await expect(
      moderator.getByText("There are no pending comment reports."),
    ).toBeVisible();
    await moderator
      .getByLabel("Comment report status")
      .selectOption("reviewing");
    await expect(moderator.getByText("Updated listening note")).toBeVisible();
    await moderator
      .getByLabel(`Moderation note for report ${reportId}`)
      .fill("Visibility updated");
    await moderator.getByRole("button", { name: "Resolve" }).click();
    await expect(
      moderator.getByText("There are no reviewing comment reports."),
    ).toBeVisible();
    expect(gateway.reports[0].status).toBe("resolved");

    await anonymous.reload();
    await expect(anonymous.getByText("No comments yet.")).toBeVisible();
    await expect(anonymous.getByText("Updated listening note")).toHaveCount(0);
    await reporter.reload();
    await expect(reporter.getByText("No comments yet.")).toBeVisible();
    expect(gateway.visibleComments()).toHaveLength(0);

    const moderationRequest = gateway.requests.find(
      ({ actor, request }) =>
        actor === "moderator" &&
        request.method() === "POST" &&
        new URL(request.url()).pathname ===
          `/v1/moderation/comment-reports/${reportId}/actions` &&
        (request.postDataJSON() as { action: string }).action ===
          "hide_comment",
    );
    expect(moderationRequest?.request.headers().authorization).toBe(
      "Bearer community-moderator-token",
    );
    expect(moderationRequest?.request.postDataJSON()).toEqual({
      action: "hide_comment",
      note: "Verified by moderator",
    });

    const unauthenticatedReads = gateway.requests.filter(
      ({ request }) =>
        request.method() === "GET" &&
        new URL(request.url()).pathname ===
          `/v1/tracks/${communityIds.track}/comments` &&
        !request.headers().authorization,
    );
    expect(unauthenticatedReads.length).toBeGreaterThan(0);
  } finally {
    await Promise.all([
      anonymousContext.close(),
      authorContext.close(),
      reporterContext.close(),
      moderatorContext.close(),
    ]);
  }
});

test("failed comment like rolls back and unauthorized mutations are rejected", async ({
  browser,
}) => {
  test.setTimeout(60_000);
  const gateway = createCommunityJourneyMock();
  const authorContext = await browser.newContext({ baseURL: appURL });
  const reporterContext = await browser.newContext({ baseURL: appURL });
  try {
    const author = await authorContext.newPage();
    const reporter = await reporterContext.newPage();
    await gateway.attach(author, "author");
    await gateway.attach(reporter, "reporter");
    await signIn(author, "author");
    await author.goto(trackPath);
    await author.getByLabel("Write a comment").fill("A comment to like");
    await author.getByRole("button", { name: "Post comment" }).click();
    await expect(ownComment(author, "A comment to like")).toBeVisible();

    await signIn(reporter, "reporter");
    await reporter.goto(trackPath);
    const comment = reporter
      .getByRole("article")
      .filter({ hasText: "A comment to like" });
    const like = comment.getByRole("button", { name: "Like comment" });
    await expect(like).toBeEnabled();
    gateway.failNextCommentLike();
    await like.click();
    await expect(reporter.getByRole("alert")).toContainText("like_unavailable");
    await expect(like).toHaveAttribute("aria-pressed", "false");
    // The next attempt is allowed and persists.
    await like.click();
    await expect(
      comment.getByRole("button", { name: "Unlike comment" }),
    ).toBeVisible();
    await reporter.reload();
    await expect(
      reporter
        .getByRole("article")
        .filter({ hasText: "A comment to like" })
        .getByRole("button", { name: "Unlike comment" }),
    ).toBeVisible();

    const id = gateway.visibleComments()[0].id;
    const rejection = await reporter.evaluate(
      async ({ base, commentId, token }) => {
        const response = await fetch(`${base}/v1/comments/${commentId}`, {
          method: "PATCH",
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ content: "A forbidden edit" }),
        });
        return response.status;
      },
      { base: gatewayURL, commentId: id, token: "community-reporter-token" },
    );
    expect(rejection).toBe(403);
    expect(gateway.visibleComments()[0].content).toBe("A comment to like");
  } finally {
    await Promise.all([authorContext.close(), reporterContext.close()]);
  }
});
