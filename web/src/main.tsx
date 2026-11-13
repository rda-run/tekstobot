import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Toasty } from "@cloudflare/kumo";
import { App } from "./App";
import { toasts } from "./avisos";
import { applyPreferences } from "./preferencias";
import "./estilo.css";

applyPreferences();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Toasty toastManager={toasts}>
      <App />
    </Toasty>
  </StrictMode>,
);
