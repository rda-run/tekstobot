// Cliente da API JSON do servidor Go (internal/ui/handlers.go).

export type Info = { version: string; migrationError: string; userName: string; authEnabled: boolean };
export type Attempt = { id: number; phoneNumber: string; pushName: string; lastAttempt: string };
export type Status = { connected: boolean; hasQR: boolean; attempts: Attempt[] };
export type Phone = { id: number; phoneNumber: string; description: string };
export type Media = {
  id: number;
  mediaType: string;
  filePath: string;
  extractedText: string;
  senderPhone: string;
  senderName: string;
  status: "pending" | "error" | "completed";
  errorMessage: string;
  createdAt: string;
};
export type MediaUser = { phoneNumber: string; name: string };

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const resp = await fetch(path, {
    method,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: "same-origin",
  });
  if (resp.status === 401) {
    // Sessão expirada: o login é uma navegação de página inteira (OIDC).
    location.assign("/auth/login");
    throw new Error("session expired");
  }
  if (!resp.ok) throw new Error((await resp.text()).trim() || `error ${resp.status}`);
  if (resp.status === 204) return undefined as T;
  return resp.json();
}

export const api = {
  info: () => req<Info>("GET", "/api/info"),
  status: () => req<Status>("GET", "/api/status"),
  resetWhatsApp: () => req<void>("POST", "/api/whatsapp/reset"),
  phones: () => req<Phone[]>("GET", "/api/phones"),
  addPhone: (phone: string, description: string) => req<void>("POST", "/api/phones", { phone, description }),
  deletePhone: (id: number) => req<void>("DELETE", `/api/phones/${id}`),
  media: (phone: string) =>
    req<{ items: Media[]; users: MediaUser[] }>("GET", `/api/media?phone=${encodeURIComponent(phone)}`),
  deleteMedia: (id: number) => req<void>("DELETE", `/api/media/${id}`),
  authorize: (phone: string, name: string) => req<void>("POST", "/api/unauthorized/authorize", { phone, name }),
  ignoreAttempt: (phone: string) => req<void>("DELETE", `/api/unauthorized/${encodeURIComponent(phone)}`),
};
