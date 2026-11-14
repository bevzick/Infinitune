import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { CoverUploadManager } from "./CoverUploadManager";

const api = vi.hoisted(() => ({
  createCoverUploadSession: vi.fn(),
  uploadCoverFile: vi.fn(),
}));

vi.mock("./creatorCoverApi", () => api);

const releaseId = "00000000-0000-4000-8000-000000000501";
const uploadId = "00000000-0000-4000-8000-000000000551";

beforeEach(() => {
  Object.defineProperty(URL, "createObjectURL", {
    configurable: true,
    value: vi.fn(() => "blob:preview"),
  });
  Object.defineProperty(URL, "revokeObjectURL", {
    configurable: true,
    value: vi.fn(),
  });
  api.createCoverUploadSession.mockResolvedValue({
    id: uploadId,
    release_id: releaseId,
    upload_url: `/v1/media/cover-uploads/${uploadId}`,
    status: "pending",
    mime_type: "image/png",
  });
  api.uploadCoverFile.mockResolvedValue({
    release_id: releaseId,
    status: "completed",
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("CoverUploadManager", () => {
  it("previews the image and refreshes the release after upload", async () => {
    const user = userEvent.setup();
    const onCompleted = vi.fn().mockResolvedValue(undefined);
    const { unmount } = render(
      <CoverUploadManager
        accessToken="token"
        releaseId={releaseId}
        hasCover={false}
        onCompleted={onCompleted}
      />,
    );
    const file = new File(["image"], "cover.png", { type: "image/png" });
    await user.upload(screen.getByLabelText("Cover image"), file);
    expect(
      await screen.findByAltText("Selected cover preview"),
    ).toHaveAttribute("src", "blob:preview");
    await user.click(screen.getByRole("button", { name: "Upload cover" }));
    expect(api.createCoverUploadSession).toHaveBeenCalledWith(
      "token",
      releaseId,
      {
        idempotency_key: expect.stringMatching(/^[0-9a-f-]{36}$/i),
        original_name: "cover.png",
        mime_type: "image/png",
        size_bytes: 5,
      },
    );
    expect(api.uploadCoverFile).toHaveBeenCalledWith(
      "token",
      expect.objectContaining({ id: uploadId }),
      file,
      expect.any(Function),
    );
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Cover uploaded successfully.",
    );
    expect(onCompleted).toHaveBeenCalledOnce();
    unmount();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:preview");
  });

  it("shows backend errors and allows a new upload attempt", async () => {
    const user = userEvent.setup();
    api.uploadCoverFile
      .mockRejectedValueOnce(new Error("temporary failure"))
      .mockResolvedValueOnce({ release_id: releaseId, status: "completed" });
    render(
      <CoverUploadManager
        accessToken="token"
        releaseId={releaseId}
        hasCover={true}
        onCompleted={vi.fn().mockResolvedValue(undefined)}
      />,
    );
    await user.upload(
      screen.getByLabelText("Cover image"),
      new File(["image"], "cover.png", { type: "image/png" }),
    );
    const upload = screen.getByRole("button", { name: "Replace cover" });
    await user.click(upload);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Could not upload cover",
    );
    expect(upload).toBeEnabled();
    await user.click(upload);
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Cover uploaded successfully.",
    );
    expect(api.createCoverUploadSession).toHaveBeenCalledTimes(2);
  });

  it("rejects invalid or empty images before creating a session", async () => {
    const user = userEvent.setup({ applyAccept: false });
    render(
      <CoverUploadManager
        accessToken="token"
        releaseId={releaseId}
        hasCover={false}
        onCompleted={vi.fn()}
      />,
    );
    await user.upload(
      screen.getByLabelText("Cover image"),
      new File(["text"], "notes.txt", { type: "text/plain" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Choose a JPEG, PNG or WebP image.",
    );
    expect(screen.getByRole("button", { name: "Upload cover" })).toBeDisabled();
    expect(api.createCoverUploadSession).not.toHaveBeenCalled();
  });
});
