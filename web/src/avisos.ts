import { createKumoToastManager } from "@cloudflare/kumo";

export const toasts = createKumoToastManager();

export function showError(e: unknown) {
  toasts.add({ title: e instanceof Error ? e.message : String(e), variant: "error" });
}
