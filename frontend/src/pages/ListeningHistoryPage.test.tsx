import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ListeningHistoryPage } from "./ListeningHistoryPage";

const historyApi = vi.hoisted(() => ({
  listListeningHistory: vi.fn(),
  clearListeningHistory: vi.fn(),
}));
const catalog = vi.hoisted(() => ({ fetchCatalogItem: vi.fn() }));

vi.mock("../features/history/historyApi", async (importOriginal) => ({
  ...(await importOriginal()),
  ...historyApi,
}));
vi.mock("../features/catalog/catalogApi", async (importOriginal) => ({
  ...(await importOriginal()),
  ...catalog,
}));
vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({
    accessToken: "private-token",
    identity: { userId: "user-1" },
  }),
}));

const trackId = "11111111-1111-4111-8111-111111111111";
const first = {
  event_id: "event-1",
  track_id: trackId,
  started_at: "2026-11-10T13:00:00.000Z",
  finished_at: "2026-11-10T13:01:00.000Z",
  listened_ms: 45000,
  completed: true,
  context_type: "search",
  context_id: null,
  device_type: "web",
};
const second = { ...first, event_id: "event-2", completed: false };

function mount() {
  render(
    <MemoryRouter>
      <ListeningHistoryPage />
    </MemoryRouter>,
  );
}

describe("ListeningHistoryPage", () => {
  beforeEach(() => {
    historyApi.listListeningHistory.mockResolvedValue({ items: [first] });
    historyApi.clearListeningHistory.mockResolvedValue({ deleted: 1 });
    catalog.fetchCatalogItem.mockResolvedValue({ title: "Night Signal" });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("renders private events and resolves track titles without leaking user ids", async () => {
    mount();
    expect(
      await screen.findByRole("link", { name: "Night Signal" }),
    ).toHaveAttribute("href", `/tracks/${trackId}`);
    expect(screen.getByText("0:45 listened")).toBeInTheDocument();
    expect(
      within(
        screen.getByRole("region", { name: "Listening events" }),
      ).getByText("Completed"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Profile settings" }),
    ).toHaveAttribute("href", "/profile");
    expect(historyApi.listListeningHistory).toHaveBeenCalledWith(
      "private-token",
      {},
      undefined,
      expect.any(AbortSignal),
    );
  });

  it("supports cursor pagination with no duplicated events", async () => {
    historyApi.listListeningHistory
      .mockResolvedValueOnce({ items: [first], next_cursor: "cursor-A" })
      .mockResolvedValueOnce({ items: [first, second] });
    const user = userEvent.setup();
    mount();
    await screen.findByRole("button", { name: "Load more" });
    await user.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() =>
      expect(
        screen.getByText("2 shown · up to 20 per page"),
      ).toBeInTheDocument(),
    );
    expect(historyApi.listListeningHistory).toHaveBeenLastCalledWith(
      "private-token",
      {},
      "cursor-A",
      expect.any(AbortSignal),
    );
    expect(
      within(
        screen.getByRole("region", { name: "Listening events" }),
      ).getAllByRole("listitem"),
    ).toHaveLength(2);
  });

  it("applies validated filters only when submitted", async () => {
    historyApi.listListeningHistory.mockResolvedValue({ items: [] });
    const user = userEvent.setup();
    mount();
    await screen.findByText("No listening events match these filters.");
    await user.selectOptions(screen.getByLabelText("Playback"), "false");
    await user.selectOptions(screen.getByLabelText("Context"), "playlist");
    expect(historyApi.listListeningHistory).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "Apply filters" }));
    await waitFor(() =>
      expect(historyApi.listListeningHistory).toHaveBeenLastCalledWith(
        "private-token",
        { completed: false, contextType: "playlist" },
        undefined,
        expect.any(AbortSignal),
      ),
    );
    await user.type(screen.getByLabelText("Track ID"), "invalid");
    await user.click(screen.getByRole("button", { name: "Apply filters" }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Track ID must be a UUID.",
    );
  });

  it("clears history only after confirmation and reloads on success", async () => {
    const confirm = vi
      .fn()
      .mockReturnValueOnce(false)
      .mockReturnValueOnce(true);
    vi.stubGlobal("confirm", confirm);
    const user = userEvent.setup();
    mount();
    await screen.findByText("0:45 listened");
    await user.click(screen.getByRole("button", { name: "Clear history" }));
    expect(historyApi.clearListeningHistory).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Clear history" }));
    await waitFor(() =>
      expect(historyApi.clearListeningHistory).toHaveBeenCalledWith(
        "private-token",
        undefined,
      ),
    );
    expect(
      await screen.findByText("1 listening event(s) deleted."),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(historyApi.listListeningHistory).toHaveBeenCalledTimes(2),
    );
  });

  it("keeps events on clear failure and offers retry for failed loads", async () => {
    historyApi.listListeningHistory.mockRejectedValueOnce(
      new Error("History unavailable"),
    );
    historyApi.clearListeningHistory.mockRejectedValueOnce(
      new Error("Permission denied"),
    );
    const confirm = vi.fn().mockReturnValue(true);
    vi.stubGlobal("confirm", confirm);
    const user = userEvent.setup();
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "History unavailable",
    );
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("0:45 listened")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Clear history" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Permission denied",
    );
    expect(screen.getByText("0:45 listened")).toBeInTheDocument();
  });
});
