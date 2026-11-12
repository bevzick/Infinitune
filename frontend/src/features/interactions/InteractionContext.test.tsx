import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { InteractionButton } from "./InteractionButton";
import { InteractionProvider } from "./InteractionContext";

const interactionApi = vi.hoisted(() => ({
  listInteractionIDs: vi.fn(),
  setInteraction: vi.fn(),
}));

vi.mock("./interactionApi", () => interactionApi);
vi.mock("../session/SessionContext", () => ({
  useSession: () => ({ accessToken: "access-token" }),
}));

describe("InteractionProvider", () => {
  beforeEach(() => {
    interactionApi.listInteractionIDs.mockResolvedValue([]);
    interactionApi.setInteraction.mockResolvedValue(undefined);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("optimistically likes a track and persists the change", async () => {
    const user = userEvent.setup();
    renderButton();
    const button = screen.getByRole("button", { name: "Like Signal" });
    await waitFor(() => expect(button).toBeEnabled());

    await user.click(button);

    expect(button).toHaveAttribute("aria-pressed", "true");
    expect(interactionApi.setInteraction).toHaveBeenCalledWith(
      "access-token",
      "track",
      "track-id",
      true,
    );
  });

  it("rolls the optimistic state back when the request fails", async () => {
    interactionApi.setInteraction.mockRejectedValueOnce(new Error("offline"));
    const user = userEvent.setup();
    renderButton();
    const button = screen.getByRole("button", { name: "Like Signal" });
    await waitFor(() => expect(button).toBeEnabled());

    await user.click(button);

    await waitFor(() =>
      expect(button).toHaveAttribute("aria-pressed", "false"),
    );
    expect(button).toHaveAttribute(
      "title",
      "Could not update your library. Try again.",
    );
  });
});

function renderButton() {
  render(
    <InteractionProvider>
      <InteractionButton kind="track" id="track-id" label="Signal" />
    </InteractionProvider>,
  );
}
