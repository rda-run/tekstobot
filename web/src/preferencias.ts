// Tema (claro/escuro) deste aparelho: fica no localStorage e vira atributo do <html>.
import { useSyncExternalStore } from "react";

export type Theme = "light" | "dark";

const KEY = "tekstobot-theme";
const BAR_COLOR: Record<Theme, string> = { light: "#F7FAFB", dark: "#0B0F13" };

const system = window.matchMedia("(prefers-color-scheme: dark)");
const listeners = new Set<() => void>();

function saved(): string | null {
  try {
    return localStorage.getItem(KEY);
  } catch {
    return null;
  }
}

// Sem escolha salva, o tema segue o sistema.
function current(): Theme {
  const s = saved();
  if (s === "light" || s === "dark") return s;
  return system.matches ? "dark" : "light";
}

let theme = current();

function apply() {
  const html = document.documentElement;
  // O Kumo troca de tema pelo atributo data-mode.
  html.dataset.mode = theme;
  html.style.colorScheme = theme;
  document.querySelectorAll('meta[name="theme-color"]').forEach((m) => {
    m.setAttribute("content", BAR_COLOR[theme]);
    m.removeAttribute("media");
  });
  listeners.forEach((f) => f());
}

// Chamado em main.tsx antes de renderizar, para não piscar o tema errado.
export function applyPreferences() {
  apply();
  system.addEventListener("change", () => {
    if (saved() === null) {
      theme = current();
      apply();
    }
  });
}

function subscribe(f: () => void) {
  listeners.add(f);
  return () => listeners.delete(f);
}

export function useTheme(): [Theme, () => void] {
  const value = useSyncExternalStore(subscribe, () => theme);
  const toggle = () => {
    theme = theme === "dark" ? "light" : "dark";
    try {
      localStorage.setItem(KEY, theme);
    } catch {
      // Sem armazenamento (modo privado etc.): vale só até recarregar.
    }
    apply();
  };
  return [value, toggle];
}
