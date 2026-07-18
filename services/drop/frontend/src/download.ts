import type { Attachment } from "@/types";

type ShareResult = "shared" | "prepared" | "unsupported";
type StandaloneNavigator = Navigator & { standalone?: boolean };

const preparedFiles = new Map<string, File>();

function isIOS(): boolean {
  return /iPad|iPhone|iPod/.test(navigator.userAgent)
    || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
}

export function usesNativeFileShare(): boolean {
  const standalone = window.matchMedia("(display-mode: standalone)").matches
    || (navigator as StandaloneNavigator).standalone === true;
  return isIOS() && standalone && typeof navigator.share === "function";
}

export async function shareAttachmentFile(attachment: Attachment): Promise<ShareResult> {
  if (!usesNativeFileShare()) return "unsupported";

  let file = preparedFiles.get(attachment.id);
  if (!file) {
    const response = await fetch(`${attachment.download_url}?download=1`, {
      credentials: "same-origin",
    });
    if (!response.ok) throw new Error(`下载失败 (${response.status})`);

    const blob = await response.blob();
    file = new File([blob], attachment.original_name, {
      type: blob.type || attachment.mime_type || "application/octet-stream",
      lastModified: Date.now(),
    });
    if (typeof navigator.canShare === "function" && !navigator.canShare({ files: [file] })) {
      return "unsupported";
    }
    preparedFiles.set(attachment.id, file);
  }

  // Fetching a large file can outlive Safari's transient user activation.
  // Keep the prepared File so a second tap can share it immediately.
  if (navigator.userActivation && !navigator.userActivation.isActive) return "prepared";

  await navigator.share({ files: [file] });
  preparedFiles.delete(attachment.id);
  return "shared";
}

export function isShareCancelled(reason: unknown): boolean {
  return reason instanceof DOMException && reason.name === "AbortError";
}
