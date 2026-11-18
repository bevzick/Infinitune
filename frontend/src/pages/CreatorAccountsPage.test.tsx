import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { CreatorAccountsPage } from "./CreatorAccountsPage";

const creatorApi = vi.hoisted(() => ({
  listCreatorAccounts: vi.fn(),
  createCreatorAccount: vi.fn(),
  submitCreatorVerification: vi.fn(),
  uploadArtistImage: vi.fn(),
}));

vi.mock("../features/creator/creatorReleaseApi", () => creatorApi);
const membersApi = vi.hoisted(() => ({
  listCreatorMembers: vi.fn(),
  addCreatorMember: vi.fn(),
  changeCreatorMemberRole: vi.fn(),
  removeCreatorMember: vi.fn(),
}));
vi.mock("../features/creator/creatorMembersApi", () => membersApi);
vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({
    accessToken: "access-token",
    identity: { userId: "00000000-0000-4000-8000-000000000001" },
  }),
}));

const account = {
  id: "creator-1",
  type: "label",
  display_name: "Northern Lights",
  verification_status: "unverified",
  image_url: "",
};

describe("CreatorAccountsPage", () => {
  beforeEach(() => {
    creatorApi.listCreatorAccounts.mockResolvedValue([]);
    creatorApi.createCreatorAccount.mockResolvedValue(account);
    membersApi.listCreatorMembers.mockResolvedValue([
      {
        user_id: "00000000-0000-4000-8000-000000000001",
        role: "owner",
        created_at: "2026-11-13T00:00:00Z",
      },
    ]);
    creatorApi.submitCreatorVerification.mockResolvedValue({
      ...account,
      verification_status: "pending",
    });
    creatorApi.uploadArtistImage.mockResolvedValue({
      artist_id: "artist-1",
      image_url: "/v1/media/artists/artist-1/image",
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("creates a creator identity and submits it for verification", async () => {
    const user = userEvent.setup();
    render(<CreatorAccountsPage />);

    await user.click(
      await screen.findByRole("button", { name: "+ New creator account" }),
    );
    await user.selectOptions(screen.getByLabelText("Account type"), "label");
    await user.type(screen.getByLabelText("Display name"), "Northern Lights");
    await user.click(screen.getByRole("button", { name: "Create account" }));

    expect(creatorApi.createCreatorAccount).toHaveBeenCalledWith(
      "access-token",
      { type: "label", display_name: "Northern Lights" },
    );
    expect(await screen.findByText("Not submitted")).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Submit for verification" }),
    );
    expect(creatorApi.submitCreatorVerification).toHaveBeenCalledWith(
      "access-token",
      "creator-1",
    );
    expect(await screen.findByText("Pending review")).toBeInTheDocument();
    expect(screen.getByText("Awaiting moderator review")).toBeInTheDocument();
  });

  it("opens creator member management from the account card", async () => {
    const user = userEvent.setup();
    creatorApi.listCreatorAccounts.mockResolvedValue([account]);
    render(<CreatorAccountsPage />);
    await user.click(
      await screen.findByRole("button", { name: "Manage members" }),
    );
    expect(
      await screen.findByRole("heading", { name: "Team members" }),
    ).toBeInTheDocument();
    expect(membersApi.listCreatorMembers).toHaveBeenCalledWith(
      "access-token",
      "creator-1",
      expect.any(AbortSignal),
    );
    await user.click(screen.getByRole("button", { name: "Close members" }));
    expect(
      screen.queryByRole("heading", { name: "Team members" }),
    ).not.toBeInTheDocument();
  });

  it("uploads an image for an artist account", async () => {
    const user = userEvent.setup();
    creatorApi.listCreatorAccounts.mockResolvedValue([
      { ...account, type: "artist", display_name: "Luna Veil" },
    ]);
    render(<CreatorAccountsPage />);
    const input = await screen.findByLabelText("Upload artist image");
    const file = new File([new Uint8Array([1, 2, 3])], "artist.png", {
      type: "image/png",
    });

    await user.upload(input, file);

    expect(creatorApi.uploadArtistImage).toHaveBeenCalledWith(
      "access-token",
      "creator-1",
      file,
    );
    expect(await screen.findByText(/Image for/)).toBeInTheDocument();
  });
});
