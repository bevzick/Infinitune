import { afterEach, describe, expect, it, vi } from "vitest";

import {
  type CoverUploadSession,
  createCoverUploadSession,
  uploadCoverFile,
} from "./creatorCoverApi";

const session: CoverUploadSession = {
  id: "00000000-0000-4000-8000-000000000551",
  release_id: "00000000-0000-4000-8000-000000000501",
  upload_url: "/v1/media/cover-uploads/00000000-0000-4000-8000-000000000551",
  original_name: "cover.png",
  mime_type: "image/png",
  size_bytes: 5,
  status: "pending",
};

type Listener = () => void;

class FakeUploadRequest {
  static latest: FakeUploadRequest;
  status = 0;
  response: unknown = null;
  responseType = "";
  withCredentials = false;
  method = "";
  url = "";
  body?: File;
  headers: Record<string, string> = {};
  listeners: Record<string, Listener> = {};
  upload = { addEventListener: vi.fn() };

  constructor() {
    FakeUploadRequest.latest = this;
  }

  open(method: string, url: string) {
    this.method = method;
    this.url = url;
  }

  setRequestHeader(header: string, value: string) {
    this.headers[header] = value;
  }

  addEventListener(name: string, listener: Listener) {
    this.listeners[name] = listener;
  }

  send(file: File) {
    this.body = file;
  }

  complete(status: number, response: unknown) {
    this.status = status;
    this.response = response;
    this.listeners.load?.();
  }
}

afterEach(() => vi.unstubAllGlobals());

describe("creator cover upload API", () => {
  it("creates a cover session using the Content API", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(session), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    await createCoverUploadSession("token", session.release_id, {
      idempotency_key: "00000000-0000-4000-8000-000000000552",
      original_name: "cover.png",
      mime_type: "image/png",
      size_bytes: 5,
    });
    expect(fetchMock.mock.calls[0][0]).toBe(
      `http://localhost:8000/v1/releases/${session.release_id}/cover-uploads`,
    );
    expect(fetchMock.mock.calls[0][1]).toMatchObject({
      method: "POST",
      credentials: "include",
    });
    expect(fetchMock.mock.calls[0][1].headers.get("Authorization")).toBe(
      "Bearer token",
    );
    expect(JSON.parse(fetchMock.mock.calls[0][1].body).size_bytes).toBe(5);
  });

  it("sends raw bytes to the session URL with Bearer authorization", async () => {
    vi.stubGlobal("XMLHttpRequest", FakeUploadRequest);
    const file = new File(["image"], "cover.png", { type: "image/png" });
    const onProgress = vi.fn();
    const pending = uploadCoverFile("token", session, file, onProgress);
    const xhr = FakeUploadRequest.latest;
    expect(xhr.method).toBe("PUT");
    expect(xhr.url).toBe(`http://localhost:8000${session.upload_url}`);
    expect(xhr.headers.Authorization).toBe("Bearer token");
    expect(xhr.headers["Content-Type"]).toBe("image/png");
    expect(xhr.body).toBe(file);
    xhr.complete(201, { release_id: session.release_id, status: "completed" });
    await expect(pending).resolves.toEqual({
      release_id: session.release_id,
      status: "completed",
    });
    expect(onProgress).toHaveBeenCalledWith(100);
  });

  it("rejects media errors and invalid success responses", async () => {
    vi.stubGlobal("XMLHttpRequest", FakeUploadRequest);
    const file = new File(["image"], "cover.png", { type: "image/png" });
    const failed = uploadCoverFile("token", session, file, vi.fn());
    FakeUploadRequest.latest.complete(422, {
      error: { code: "invalid_cover", message: "Image data invalid" },
    });
    await expect(failed).rejects.toMatchObject({
      status: 422,
      code: "invalid_cover",
    });
    const badResponse = uploadCoverFile("token", session, file, vi.fn());
    FakeUploadRequest.latest.complete(200, { status: "pending" });
    await expect(badResponse).rejects.toMatchObject({
      code: "invalid_cover_upload_response",
    });
  });

  it("never uploads to an unexpected URL", async () => {
    const file = new File(["image"], "cover.png", { type: "image/png" });
    await expect(
      uploadCoverFile(
        "token",
        { ...session, upload_url: "//untrusted.example/steal" },
        file,
        vi.fn(),
      ),
    ).rejects.toThrow("Invalid cover upload URL");
  });
});
