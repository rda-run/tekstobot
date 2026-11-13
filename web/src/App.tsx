import { useCallback, useEffect, useRef, useState } from "react";
import { Badge, Button, Dialog, Input, Loader, Select } from "@cloudflare/kumo";
import {
  CaretDownIcon,
  CaretUpIcon,
  CheckIcon,
  MoonIcon,
  PlusIcon,
  SunIcon,
  TrashIcon,
  UsersIcon,
  WarningIcon,
  XIcon,
} from "@phosphor-icons/react";
import { api, type Info, type Media, type MediaUser, type Phone, type Status } from "./api";
import { showError } from "./avisos";
import { useTheme } from "./preferencias";

const ALL = "all"; // valor do Select para "todos os usuários"

function when(iso: string) {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "short", timeStyle: "short" });
}

export function App() {
  const [info, setInfo] = useState<Info | null>(null);
  const [status, setStatus] = useState<Status | null>(null);
  const [media, setMedia] = useState<Media[] | null>(null);
  const [users, setUsers] = useState<MediaUser[]>([]);
  const [filter, setFilter] = useState(ALL);
  const [access, setAccess] = useState(false);
  const [tick, setTick] = useState(0); // muda a cada consulta, para o QR não ficar em cache

  const loadStatus = useCallback(
    () =>
      api.status().then((s) => {
        setStatus(s);
        setTick(Date.now());
      }),
    [],
  );
  const loadMedia = useCallback(
    () =>
      api.media(filter === ALL ? "" : filter).then((m) => {
        setMedia(m.items);
        setUsers(m.users);
      }),
    [filter],
  );

  useEffect(() => {
    api.info().then(setInfo, showError);
  }, []);

  const ready = info !== null && !info.migrationError;
  useEffect(() => {
    if (ready) loadMedia().catch(showError);
  }, [ready, loadMedia]);

  // A cada 5s: estado do WhatsApp sempre; mídias só enquanto houver transcrição pendente.
  // Falhas aqui ficam caladas: um aviso de erro a cada 5s não ajudaria ninguém.
  const pending = useRef(false);
  pending.current = media?.some((m) => m.status === "pending") ?? false;
  useEffect(() => {
    if (!ready) return;
    const poll = () => {
      loadStatus().catch(() => {});
      if (pending.current) loadMedia().catch(() => {});
    };
    poll();
    const id = setInterval(poll, 5000);
    return () => clearInterval(id);
  }, [ready, loadStatus, loadMedia]);

  if (!info) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Loader size={24} />
      </div>
    );
  }
  if (info.migrationError) return <Maintenance info={info} />;

  const reload = () => Promise.all([loadStatus(), loadMedia()]).catch(showError);
  const filterItems: Record<string, string> = { [ALL]: "All users" };
  for (const u of users) filterItems[u.phoneNumber] = u.name || u.phoneNumber;

  return (
    <div className="flex min-h-screen flex-col">
      <Header info={info} status={status} onAccess={() => setAccess(true)} />
      <main className="mx-auto w-full max-w-[1200px] flex-1 px-4 pt-8 pb-16 sm:px-8">
        <section aria-labelledby="t-history" className="overflow-hidden rounded-md border border-kumo-hairline bg-kumo-base">
          <div className="flex min-h-14 flex-wrap items-center gap-3 border-b border-kumo-tint py-2 pr-3 pl-5">
            <h1 id="t-history" className="rotulo !text-xs">Processing history</h1>
            <div className="ml-auto w-full sm:w-[220px]">
              <Select aria-label="Filter by user" className="w-full" value={filter} onValueChange={(v) => setFilter(v ?? ALL)} items={filterItems} />
            </div>
          </div>
          {media === null ? (
            <div className="flex justify-center py-12">
              <Loader size={24} />
            </div>
          ) : media.length === 0 ? (
            <div className="px-5 py-12 text-center">
              <p className="m-0 font-mono text-sm font-bold text-[var(--teksto-ink)]">No media</p>
              <p className="m-0 mt-1 text-sm text-kumo-subtle">Processed media will automatically appear here.</p>
            </div>
          ) : (
            <ul className="m-0 list-none p-0">
              {media.map((m) => (
                <MediaRow key={m.id} media={m} onDelete={() => api.deleteMedia(m.id).then(loadMedia).catch(showError)} />
              ))}
            </ul>
          )}
        </section>
      </main>
      {access && status && <AccessDialog status={status} onClose={() => setAccess(false)} onChange={reload} />}
      {status && !status.connected && <PairingDialog hasQR={status.hasQR} tick={tick} onReset={loadStatus} />}
    </div>
  );
}

// Símbolo t_ da marca (o mesmo desenho de web/public/favicon.svg).
function Mark({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 512 512" aria-hidden="true" className="shrink-0">
      <rect width="512" height="512" rx="112" fill="#0B0F13" />
      <path fill="none" stroke="#FFFFFF" strokeWidth="52" d="M210 124V372M138 208H296" />
      <rect x="318" y="320" width="110" height="52" fill="#4C6CB5" />
    </svg>
  );
}

function Header({ info, status, onAccess }: { info: Info; status: Status | null; onAccess: () => void }) {
  const [theme, toggleTheme] = useTheme();
  const pending = status?.attempts.length ?? 0;
  const themeLabel = theme === "dark" ? "Use light theme" : "Use dark theme";
  return (
    <header className="sticky top-0 z-10 border-b border-kumo-hairline bg-kumo-canvas/90 backdrop-blur">
      <div className="mx-auto flex min-h-14 w-full max-w-[1200px] flex-wrap items-center gap-x-6 gap-y-1 px-4 py-1.5 sm:px-8">
        <a href="/" className="flex items-center gap-2.5 no-underline">
          <Mark size={26} />
          <span className="font-brand text-[17px] font-bold tracking-tight text-[var(--teksto-ink)]">
            tekstobot<span className="text-kumo-brand">_</span>
          </span>
          <span className="font-mono text-[13px] text-kumo-inactive">/ rda.run</span>
        </a>
        {status && (
          <span className="flex items-center gap-2 font-mono text-xs text-kumo-subtle">
            <span className={"inline-block size-2 rounded-full " + (status.connected ? "bg-kumo-success" : "bg-kumo-danger")} />
            {status.connected ? "connected" : "disconnected"}
          </span>
        )}
        <div className="ml-auto flex items-center gap-2">
          <span className="mr-1 hidden font-mono text-xs text-kumo-inactive sm:inline">v{info.version}</span>
          <Button size="sm" variant="outline" icon={UsersIcon} onClick={onAccess}>
            Access
            {pending > 0 && <Badge variant="error">{pending}</Badge>}
          </Button>
          <Button size="sm" shape="square" variant="ghost" icon={theme === "dark" ? SunIcon : MoonIcon} aria-label={themeLabel} title={themeLabel} onClick={toggleTheme} />
          {info.authEnabled && (
            <>
              <span className="ml-1 hidden max-w-[140px] truncate font-mono text-xs text-kumo-subtle sm:inline">{info.userName}</span>
              <Button size="sm" variant="outline" onClick={() => location.assign("/auth/logout")}>
                Logout
              </Button>
            </>
          )}
        </div>
      </div>
    </header>
  );
}

// Excluir pede um segundo clique; sair do botão desfaz a confirmação.
function DeleteButton({ label, onDelete }: { label: string; onDelete: () => void }) {
  const [confirm, setConfirm] = useState(false);
  if (confirm) {
    return (
      <Button size="sm" variant="destructive" autoFocus onBlur={() => setConfirm(false)} onClick={onDelete}>
        Confirm
      </Button>
    );
  }
  return <Button shape="square" variant="ghost" icon={TrashIcon} aria-label={label} title={label} onClick={() => setConfirm(true)} />;
}

const STATUS_BADGE = { pending: "warning", error: "error", completed: "success" } as const;

function MediaRow({ media: m, onDelete }: { media: Media; onDelete: () => void }) {
  const [open, setOpen] = useState(false);
  return (
    <li className="border-b border-kumo-tint last:border-b-0">
      <div className="flex min-h-14 items-center gap-2 py-2 pr-2 pl-5">
        <button
          type="button"
          aria-expanded={open}
          onClick={() => setOpen(!open)}
          className="flex min-w-0 flex-1 cursor-pointer flex-col gap-1 border-0 bg-transparent p-0 text-left text-inherit"
        >
          <span className="flex items-center gap-2">
            <Badge variant="neutral">{m.mediaType}</Badge>
            <Badge variant={STATUS_BADGE[m.status] ?? "neutral"}>{m.status}</Badge>
          </span>
          <span className="truncate font-mono text-xs text-kumo-subtle">
            {m.senderName || m.senderPhone} · {when(m.createdAt)}
          </span>
        </button>
        <DeleteButton label="Delete this media permanently" onDelete={onDelete} />
        <Button shape="square" variant="ghost" icon={open ? CaretUpIcon : CaretDownIcon} aria-label={open ? "Collapse" : "Expand"} onClick={() => setOpen(!open)} />
      </div>
      {open && (
        <div className="flex flex-col gap-3 border-t border-kumo-tint bg-kumo-canvas px-5 py-4">
          {m.mediaType === "audio" && <audio controls preload="none" src={"/" + m.filePath} className="h-10 w-full" />}
          {m.status === "pending" ? (
            <p className="m-0 flex items-center gap-2 font-mono text-sm text-kumo-subtle">
              <Loader size={16} /> Awaiting transcription...
            </p>
          ) : (
            <p className={"m-0 font-mono text-sm break-words whitespace-pre-wrap " + (m.status === "error" ? "text-kumo-danger" : "")}>
              {m.status === "error" ? m.errorMessage : m.extractedText}
            </p>
          )}
        </div>
      )}
    </li>
  );
}

function AccessDialog({ status, onClose, onChange }: { status: Status; onClose: () => void; onChange: () => void }) {
  const [phones, setPhones] = useState<Phone[]>([]);
  const [phone, setPhone] = useState("");
  const [description, setDescription] = useState("");
  const loadPhones = useCallback(() => api.phones().then(setPhones, showError), []);
  useEffect(() => {
    loadPhones();
  }, [loadPhones]);

  // Toda alteração de acesso muda também a fila de pendentes e os nomes no histórico.
  const run = (action: Promise<void>) =>
    action.then(() => {
      loadPhones();
      onChange();
    }, showError);

  function add(e: React.FormEvent) {
    e.preventDefault();
    run(api.addPhone(phone, description)).then(() => {
      setPhone("");
      setDescription("");
    });
  }

  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog size="lg" className="p-0">
        <div className="flex items-center border-b border-kumo-tint py-4 pr-4 pl-6">
          <Dialog.Title className="font-mono text-lg font-bold text-[var(--teksto-ink)]">Access management</Dialog.Title>
          <Dialog.Close render={(p) => <Button {...p} className="ml-auto" shape="square" variant="ghost" icon={XIcon} aria-label="Close" />} />
        </div>
        <div className="flex max-h-[70vh] flex-col gap-6 overflow-y-auto px-6 py-5">
          <section className="flex flex-col gap-2">
            <h2 className="rotulo">Pending attempts</h2>
            {status.attempts.length === 0 ? (
              <p className="m-0 text-sm text-kumo-subtle">No pending attempts.</p>
            ) : (
              <ul className="m-0 list-none p-0">
                {status.attempts.map((a) => (
                  <li key={a.id} className="flex min-h-12 items-center gap-2 border-t border-kumo-tint py-2">
                    <div className="flex min-w-0 flex-1 flex-col">
                      <span className="font-mono text-sm">{a.phoneNumber}</span>
                      <span className="truncate text-xs text-kumo-subtle">
                        {a.pushName || "Unknown"} · {when(a.lastAttempt)}
                      </span>
                    </div>
                    <Button size="sm" variant="primary" icon={CheckIcon} onClick={() => run(api.authorize(a.phoneNumber, a.pushName))}>
                      Authorize
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => run(api.ignoreAttempt(a.phoneNumber))}>
                      Ignore
                    </Button>
                  </li>
                ))}
              </ul>
            )}
          </section>
          <section className="flex flex-col gap-2">
            <h2 className="rotulo">Add manually</h2>
            <form onSubmit={add} className="flex flex-wrap items-end gap-2">
              <Input label="Number" placeholder="5511..." inputMode="tel" required value={phone} onChange={(e) => setPhone(e.target.value)} className="min-w-[140px] flex-1" />
              <Input label="Name" placeholder="Name or description" value={description} onChange={(e) => setDescription(e.target.value)} className="min-w-[140px] flex-1" />
              <Button type="submit" variant="primary" icon={PlusIcon}>
                Add
              </Button>
            </form>
          </section>
          <section className="flex flex-col gap-2">
            <h2 className="rotulo">Authorized</h2>
            {phones.length === 0 ? (
              <p className="m-0 text-sm text-kumo-subtle">No registered phones.</p>
            ) : (
              <ul className="m-0 list-none p-0">
                {phones.map((p) => (
                  <li key={p.id} className="flex min-h-12 items-center gap-2 border-t border-kumo-tint py-1">
                    <div className="flex min-w-0 flex-1 flex-col">
                      <span className="font-mono text-sm">{p.phoneNumber}</span>
                      <span className="truncate text-xs text-kumo-subtle">{p.description}</span>
                    </div>
                    <DeleteButton label={`Remove ${p.phoneNumber} and its media`} onDelete={() => run(api.deletePhone(p.id))} />
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>
      </Dialog>
    </Dialog.Root>
  );
}

// Sem conexão com o WhatsApp não há o que fazer no painel: o diálogo não fecha.
function PairingDialog({ hasQR, tick, onReset }: { hasQR: boolean; tick: number; onReset: () => Promise<void> }) {
  const [resetting, setResetting] = useState(false);
  async function reset() {
    setResetting(true);
    try {
      await api.resetWhatsApp();
      await onReset();
    } catch (e) {
      showError(e);
    } finally {
      setResetting(false);
    }
  }
  return (
    <Dialog.Root open>
      <Dialog size="base" className="flex flex-col items-center gap-4 p-6 text-center outline-none">
        <Dialog.Title className="font-mono text-lg font-bold text-[var(--teksto-ink)]">{hasQR ? "WhatsApp pairing" : "WhatsApp disconnected"}</Dialog.Title>
        {hasQR ? (
          <>
            <img src={`/qr.png?v=${tick}`} alt="QR code" width={256} height={256} className="rounded border border-kumo-hairline" />
            <Dialog.Description className="text-sm text-kumo-subtle">Scan the QR code with WhatsApp on your phone to connect the bot.</Dialog.Description>
          </>
        ) : (
          <>
            <Dialog.Description className="text-sm text-kumo-subtle">
              The bot is not connected and the session expired or got stuck. Clear the session, then restart the bot to get a new QR code.
            </Dialog.Description>
            <Button variant="destructive" loading={resetting} onClick={reset}>
              Clear session
            </Button>
          </>
        )}
      </Dialog>
    </Dialog.Root>
  );
}

function Maintenance({ info }: { info: Info }) {
  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col items-center justify-center gap-4 p-6 text-center">
      <WarningIcon size={48} className="text-kumo-danger" aria-hidden="true" />
      <h1 className="m-0 font-mono text-xl font-bold text-[var(--teksto-ink)]">Critical database failure</h1>
      <p className="m-0 text-kumo-subtle">System migrations could not be applied correctly.</p>
      <pre className="m-0 w-full overflow-x-auto rounded border border-kumo-hairline bg-kumo-base p-4 text-left font-mono text-xs whitespace-pre-wrap text-kumo-danger">{info.migrationError}</pre>
      <p className="m-0 text-left text-sm text-kumo-subtle">
        The WhatsApp bot has been disabled for safety to prevent data corruption. To diagnose the problem, run on the server:
      </p>
      <code className="w-full rounded border border-kumo-hairline bg-kumo-base p-2 text-left font-mono text-xs">journalctl -u tekstobot -n 50</code>
      <p className="m-0 font-mono text-xs text-kumo-inactive">TekstoBot v{info.version} · maintenance mode</p>
    </main>
  );
}
