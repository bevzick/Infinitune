import { useEffect, useRef, useState } from "react";
import { ApiError } from "../../shared/api/client";
import { createCoverUploadSession, uploadCoverFile } from "./creatorCoverApi";

const maximumCoverBytes = 10 * 1024 * 1024;
const coverTypes: Record<string, string> = {
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  png: "image/png",
  webp: "image/webp",
};

type CoverStage = "idle" | "creating" | "uploading" | "completed" | "error";

export function CoverUploadManager({
  accessToken,
  releaseId,
  hasCover,
  onCompleted,
}: {
  accessToken: string;
  releaseId: string;
  hasCover: boolean;
  onCompleted(): Promise<void>;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState("");
  const [stage, setStage] = useState<CoverStage>("idle");
  const [progress, setProgress] = useState(0);
  const [message, setMessage] = useState("");
  const uploadActive = useRef(false);
  const busy = stage === "creating" || stage === "uploading";
  const validationError = file ? validateCoverFile(file) : "";

  useEffect(() => {
    if (!file || validateCoverFile(file)) {
      setPreviewUrl("");
      return;
    }
    const url = URL.createObjectURL(file);
    setPreviewUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  function selectFile(next: File | null) {
    uploadActive.current = false;
    setFile(next);
    setStage(next && validateCoverFile(next) ? "error" : "idle");
    setProgress(0);
    setMessage(next ? validateCoverFile(next) : "");
  }

  async function upload() {
    if (!file || busy || validateCoverFile(file)) return;
    const extension = file.name.split(".").pop()?.toLowerCase() || "";
    setStage("creating");
    setProgress(0);
    setMessage("Creating cover upload session…");
    try {
      const session = await createCoverUploadSession(accessToken, releaseId, {
        idempotency_key: crypto.randomUUID(),
        original_name: file.name,
        mime_type: coverTypes[extension],
        size_bytes: file.size,
      });
      if (session.release_id !== releaseId) {
        throw new Error("Cover session targets the wrong release");
      }
      uploadActive.current = true;
      setStage("uploading");
      setMessage("Uploading cover…");
      await uploadCoverFile(accessToken, session, file, (value) => {
        if (uploadActive.current) setProgress(value);
      });
      uploadActive.current = false;
      setStage("completed");
      setProgress(100);
      setMessage("Cover uploaded successfully.");
      try {
        await onCompleted();
      } catch {
        setMessage(
          "Cover uploaded, but release details could not be refreshed. Reload the page to confirm.",
        );
      }
    } catch (reason) {
      uploadActive.current = false;
      setStage("error");
      setProgress(0);
      setMessage(
        reason instanceof ApiError ? reason.message : "Could not upload cover",
      );
    }
  }

  return (
    <section className="creator-cover-upload" aria-label="Release cover upload">
      <header>
        <div>
          <span className="eyebrow">Cover artwork</span>
          <h3>{hasCover ? "Replace cover" : "Upload cover"}</h3>
        </div>
        <p>JPEG, PNG or WebP · maximum 10 MiB.</p>
      </header>
      {hasCover ? (
        <p className="creator-cover-upload-existing">
          A cover is stored for this release. Upload a new image to replace it.
        </p>
      ) : (
        <p className="creator-cover-upload-existing">
          No cover has been uploaded yet.
        </p>
      )}
      <div className="creator-cover-upload-body">
        {previewUrl ? (
          <img
            className="creator-cover-upload-preview"
            src={previewUrl}
            alt="Selected cover preview"
          />
        ) : (
          <div className="creator-cover-upload-placeholder" aria-hidden="true">
            ◎
          </div>
        )}
        <div className="creator-cover-upload-controls">
          <label>
            Cover image
            <input
              type="file"
              accept=".jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp"
              disabled={busy}
              onChange={(event) =>
                selectFile(event.currentTarget.files?.[0] || null)
              }
            />
          </label>
          <button
            type="button"
            disabled={!file || Boolean(validationError) || busy}
            onClick={() => void upload()}
          >
            {busy ? "Uploading…" : hasCover ? "Replace cover" : "Upload cover"}
          </button>
        </div>
      </div>
      {message ? (
        <div
          className={`creator-cover-upload-status ${stage}`}
          role={stage === "error" ? "alert" : "status"}
        >
          <span>{message}</span>
          {stage === "uploading" ? (
            <progress
              value={progress}
              max={100}
              aria-label="Cover upload progress"
            />
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

function validateCoverFile(file: File) {
  if (
    file.name.length > 255 ||
    file.name.includes("/") ||
    file.name.includes("\\")
  ) {
    return "The cover filename must be at most 255 characters without paths.";
  }
  const extension = file.name.split(".").pop()?.toLowerCase() || "";
  const mime = coverTypes[extension];
  if (!mime || (file.type && file.type !== mime)) {
    return "Choose a JPEG, PNG or WebP image.";
  }
  if (file.size < 1) return "The cover file is empty.";
  if (file.size > maximumCoverBytes) return "The cover exceeds 10 MiB.";
  return "";
}
