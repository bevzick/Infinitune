import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../shared/api/client";
import type { CreatorRelease } from "./creatorReleaseApi";
import { ReleaseLifecycleManager } from "./ReleaseLifecycleManager";

const api = vi.hoisted(() => ({
  getReleaseReadiness: vi.fn(),
  submitRelease: vi.fn(),
  withdrawRelease: vi.fn(),
  publishReleaseWithRetry: vi.fn(),
  archiveRelease: vi.fn(),
}));
vi.mock("./creatorLifecycleApi", () => api);

const release: CreatorRelease = {
  id: "release-1",
  creator_id: "creator-1",
  title: "Signals",
  release_type: "single",
  release_date: "2026-11-13",
  status: "draft",
  cover_object_key: null,
  tracks: [
    {
      id: "track-1",
      title: "Night Signal",
      track_number: 1,
      disc_number: 1,
      explicit: false,
    },
  ],
  created_at: "2026-11-13T00:00:00Z",
  updated_at: "2026-11-13T00:00:00Z",
};

const ready = {
  release_id: "release-1",
  status: "draft",
  ready_for_submission: true,
  issues: [],
};

beforeEach(() => {
  api.getReleaseReadiness.mockResolvedValue(ready);
  api.submitRelease.mockResolvedValue({ status: "pending_review" });
  api.withdrawRelease.mockResolvedValue({ status: "draft" });
  api.publishReleaseWithRetry.mockResolvedValue({ status: "published" });
  api.archiveRelease.mockResolvedValue({ status: "archived" });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

function view(status: CreatorRelease["status"] = "draft") {
  const onUpdated = vi.fn().mockResolvedValue({ ...release, status });
  const renderResult = render(
    <ReleaseLifecycleManager
      accessToken="token"
      release={{ ...release, status }}
      onUpdated={onUpdated}
    />,
  );
  return { onUpdated, ...renderResult };
}

describe("ReleaseLifecycleManager", () => {
  it("displays all blockers and prevents premature submission", async () => {
    const user = userEvent.setup();
    api.getReleaseReadiness.mockResolvedValue({
      ...ready,
      ready_for_submission: false,
      issues: [
        { code: "cover_missing", message: "cover artwork is required" },
        {
          code: "streaming_rights_missing",
          message: "worldwide streaming rights are required",
          track_id: "track-1",
        },
      ],
    });
    view();
    expect(
      await screen.findByText(/cover artwork is required/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/worldwide streaming rights are required/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Submit for review" }),
    ).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Refresh status" }));
    expect(api.getReleaseReadiness).toHaveBeenCalledTimes(2);
    expect(api.submitRelease).not.toHaveBeenCalled();
  });

  it("invalidates previous readiness when the refreshed check fails", async () => {
    const user = userEvent.setup();
    view();
    expect(
      await screen.findByRole("button", { name: "Submit for review" }),
    ).toBeEnabled();
    api.getReleaseReadiness.mockRejectedValueOnce(
      new ApiError(503, "media_unavailable", "Media is unavailable"),
    );
    await user.click(screen.getByRole("button", { name: "Refresh status" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Media is unavailable",
    );
    expect(
      screen.getByRole("button", { name: "Submit for review" }),
    ).toBeDisabled();
  });

  it("submits only after backend readiness and requests canonical refresh", async () => {
    const user = userEvent.setup();
    const { onUpdated } = view();
    await user.click(
      await screen.findByRole("button", { name: "Submit for review" }),
    );
    expect(api.submitRelease).toHaveBeenCalledWith("token", "release-1");
    expect(onUpdated).toHaveBeenCalledTimes(1);
    expect(
      await screen.findByText(/Submitted for moderation/),
    ).toBeInTheDocument();
  });

  it("allows withdrawal during review but not submission/edit actions", async () => {
    const user = userEvent.setup();
    const { onUpdated } = view("pending_review");
    expect(
      screen.queryByRole("button", { name: "Submit for review" }),
    ).not.toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Withdraw submission" }),
    );
    expect(api.withdrawRelease).toHaveBeenCalledWith("token", "release-1");
    expect(onUpdated).toHaveBeenCalledTimes(1);
    expect(api.getReleaseReadiness).not.toHaveBeenCalled();
  });

  it("publishes approved releases and does not declare success on conflict", async () => {
    const user = userEvent.setup();
    api.publishReleaseWithRetry.mockRejectedValueOnce(
      new ApiError(409, "release_not_ready", "Not ready"),
    );
    view("approved");
    await user.click(screen.getByRole("button", { name: "Publish release" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Catalog synchronization is pending",
    );
    expect(screen.queryByText(/Publication accepted/)).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Publish release" }));
    expect(await screen.findByText(/Publication accepted/)).toBeInTheDocument();
  });

  it("requires explicit confirmation before archive", async () => {
    const user = userEvent.setup();
    view("published");
    await user.click(screen.getByRole("button", { name: /^Archive$/ }));
    expect(api.archiveRelease).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Cancel archive" }));
    expect(api.archiveRelease).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: /^Archive$/ }));
    await user.click(screen.getByRole("button", { name: "Confirm archive" }));
    expect(api.archiveRelease).toHaveBeenCalledWith("token", "release-1");
  });
});
