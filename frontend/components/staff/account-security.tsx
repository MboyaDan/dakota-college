"use client";

import { QRCodeSVG } from "qrcode.react";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/field";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { api, errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatNairobi } from "@/lib/staff/datetime";
import { fields } from "@/lib/form-fields";
import { useSession } from "./session";
import { useAction } from "./use-action";

// Two-step sign-in for the signed-in member of staff (Stage 17b, D-206).
// Optional; suggested to holders of finance, registrar and admin permissions.

type Status = components["schemas"]["TwoStepStatus"];
type Setup = components["schemas"]["TwoStepSetup"];

export function AccountSecurity() {
  const { user } = useSession();
  const [status, setStatus] = useState<Status | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [codes, setCodes] = useState<string[] | null>(null);

  const load = useCallback(() => {
    void api.GET("/api/v1/auth/two-step").then(({ data, error: err }) => {
      if (data) setStatus(data);
      else setError(errorMessage(err));
    });
  }, []);
  useEffect(load, [load]);

  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Your account</h1>
        <p className="break-all text-ink-muted">
          {user.full_name} · {user.email}
        </p>
      </div>
      <section aria-labelledby="two-step-title" className="flex flex-col gap-4">
        <h2 id="two-step-title" className="text-2xl font-semibold">
          Two-step sign-in
        </h2>
        <p className="max-w-prose">
          With two-step sign-in, signing in needs your password and a 6-digit code from an authenticator app
          on your phone. Someone who learns your password still cannot get in.
        </p>
        {error ? (
          <Alert tone="danger" title="Could not load your settings">
            {error}
          </Alert>
        ) : null}
        {status === null && !error ? (
          <Loading label="Loading two-step sign-in">
            <Skeleton className="h-32 w-full" />
          </Loading>
        ) : null}
        {codes ? (
          <RecoveryCodes
            codes={codes}
            onSaved={() => {
              setCodes(null);
              load();
            }}
          />
        ) : status?.enabled ? (
          <TwoStepOn status={status} onCodes={setCodes} onOff={load} />
        ) : status ? (
          <TwoStepOff recommended={status.recommended} onOn={setCodes} />
        ) : null}
      </section>
    </div>
  );
}

function TwoStepOff({ recommended, onOn }: { recommended: boolean; onOn: (codes: string[]) => void }) {
  const [setup, setSetup] = useState<Setup | null>(null);
  const [started, setStarted] = useState(false);
  const { busy, run } = useAction();

  if (!started) {
    return (
      <Card>
        <p className="font-semibold">Two-step sign-in is off.</p>
        {recommended ? (
          <p>
            Your role can see or change money, student records or staff accounts, so we strongly suggest
            turning it on.
          </p>
        ) : null}
        <p>
          You need about two minutes and a phone with an authenticator app, such as Google Authenticator,
          Microsoft Authenticator or 2FAS.
        </p>
        <Button className="self-start" onClick={() => setStarted(true)}>
          Set up two-step sign-in
        </Button>
      </Card>
    );
  }

  if (!setup) {
    return (
      <Card>
        <PasswordForm
          intro="Step 1 of 3: confirm it is you."
          submitLabel="Continue"
          busy={busy}
          onSubmit={async (password) => {
            const res = await run(() => api.POST("/api/v1/auth/two-step/setup", { body: { password } }), "");
            if (res) setSetup(res);
          }}
          onCancel={() => setStarted(false)}
        />
      </Card>
    );
  }

  return (
    <Card>
      <form
        method="post"
        className="flex flex-col gap-5"
        onSubmit={async (e) => {
          e.preventDefault();
          const { code } = fields(e.currentTarget, "code");
          const res = await run(
            () => api.POST("/api/v1/auth/two-step/confirm", { body: { code } }),
            "Two-step sign-in is on",
          );
          if (res) onOn(res.recovery_codes);
        }}
      >
        <p className="font-semibold">Step 2 of 3: add Dakota Training College to your app.</p>
        <ol className="flex list-decimal flex-col gap-2 pl-5">
          <li>In the app, choose to add an account and scan this QR code.</li>
          <li>Type the 6-digit code the app shows.</li>
        </ol>
        <div className="flex flex-col items-start gap-4 sm:flex-row sm:items-center">
          {/* White margin: scanners need a quiet zone round the code. */}
          <div className="rounded-card border border-stone bg-white p-3">
            <QRCodeSVG
              value={setup.otpauth_uri}
              size={176}
              level="M"
              title="QR code for your authenticator app"
            />
          </div>
          <div className="flex min-w-0 flex-col gap-1">
            <p className="text-sm">Cannot scan? Choose to enter a key, and type:</p>
            <p
              className="font-mono text-lg break-words"
              aria-label={`Key: ${setup.secret.split(" ").join(", ")}`}
            >
              {setup.secret}
            </p>
            <p className="text-sm text-ink-muted">Time-based, 6 digits. Keep this key private.</p>
          </div>
        </div>
        <Input
          name="code"
          label="6-digit code from the app"
          inputMode="numeric"
          autoComplete="one-time-code"
          pattern="[0-9 ]{6,7}"
          maxLength={7}
          required
          className="font-mono tracking-widest sm:max-w-48"
        />
        <div className="flex flex-wrap gap-3">
          <Button type="submit" loading={busy}>
            Turn on
          </Button>
          <Button type="button" variant="outline" onClick={() => setStarted(false)}>
            Cancel
          </Button>
        </div>
      </form>
    </Card>
  );
}

function RecoveryCodes({ codes, onSaved }: { codes: string[]; onSaved: () => void }) {
  const [copied, setCopied] = useState(false);
  const text = `Dakota Training College staff portal: recovery codes\nEach code works once.\n\n${codes.join("\n")}\n`;

  return (
    <Card>
      <p className="font-semibold">Save your recovery codes.</p>
      <p>
        If you lose your phone, sign in with one of these codes instead of a code from the app. Each code
        works once. Keep them somewhere safe, such as a password manager or a printed copy in a locked drawer.
        You will not see them again.
      </p>
      <ul className="grid grid-cols-2 gap-x-6 gap-y-1 rounded-card border border-stone bg-ivory p-4 font-mono sm:grid-cols-2">
        {codes.map((c) => (
          <li key={c}>{c}</li>
        ))}
      </ul>
      <div className="flex flex-wrap gap-3">
        <Button
          type="button"
          variant="secondary"
          onClick={() => {
            void navigator.clipboard?.writeText(text).then(() => setCopied(true));
          }}
        >
          {copied ? "Copied" : "Copy codes"}
        </Button>
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            const a = document.createElement("a");
            a.href = URL.createObjectURL(new Blob([text], { type: "text/plain" }));
            a.download = "dakota-recovery-codes.txt";
            a.click();
            URL.revokeObjectURL(a.href);
          }}
        >
          Download as a file
        </Button>
      </div>
      <Button type="button" className="self-start" onClick={onSaved}>
        I have saved my codes
      </Button>
    </Card>
  );
}

function TwoStepOn({
  status,
  onCodes,
  onOff,
}: {
  status: Status;
  onCodes: (codes: string[]) => void;
  onOff: () => void;
}) {
  const [open, setOpen] = useState<"codes" | "off" | null>(null);
  const { busy, run } = useAction();
  const low = status.recovery_codes_left <= 3;

  return (
    <Card>
      <p className="font-semibold">
        Two-step sign-in is on{status.enabled_at ? ` (since ${formatNairobi(status.enabled_at)})` : ""}.
      </p>
      <p className={low ? "font-semibold text-danger" : undefined}>
        {status.recovery_codes_left === 1
          ? "1 recovery code left."
          : `${status.recovery_codes_left} recovery codes left.`}
        {low ? " Get new codes now, so you can still sign in if you lose your phone." : ""}
      </p>
      {open === null ? (
        <div className="flex flex-wrap gap-3">
          <Button type="button" variant="secondary" onClick={() => setOpen("codes")}>
            Get new recovery codes
          </Button>
          <Button type="button" variant="outline" onClick={() => setOpen("off")}>
            Turn off
          </Button>
        </div>
      ) : (
        <Panel>
          <PasswordForm
            intro={
              open === "codes"
                ? "New codes replace all your old ones, used or not."
                : "To move to a new phone, turn two-step sign-in off and set it up again. While it is off, your password alone signs you in."
            }
            submitLabel={open === "codes" ? "Get new codes" : "Turn off two-step sign-in"}
            danger={open === "off"}
            busy={busy}
            onSubmit={async (password) => {
              if (open === "codes") {
                const res = await run(
                  () => api.POST("/api/v1/auth/two-step/recovery-codes", { body: { password } }),
                  "New recovery codes made",
                );
                if (res) onCodes(res.recovery_codes);
              } else {
                const res = await run(
                  () => api.POST("/api/v1/auth/two-step/disable", { body: { password } }),
                  "Two-step sign-in is off",
                );
                if (res) onOff();
              }
            }}
            onCancel={() => setOpen(null)}
          />
        </Panel>
      )}
    </Card>
  );
}

function Panel({ children }: { children: ReactNode }) {
  return <div className="rounded-card border border-stone bg-ivory p-4">{children}</div>;
}

function PasswordForm({
  intro,
  submitLabel,
  danger = false,
  busy,
  onSubmit,
  onCancel,
}: {
  intro: string;
  submitLabel: string;
  danger?: boolean;
  busy: boolean;
  onSubmit: (password: string) => Promise<void>;
  onCancel: () => void;
}) {
  return (
    <form
      method="post"
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        void onSubmit(fields(e.currentTarget, "password").password);
      }}
    >
      <p>{intro}</p>
      <Input
        name="password"
        label="Your password"
        type="password"
        autoComplete="current-password"
        required
        className="sm:max-w-sm"
      />
      <div className="flex flex-wrap gap-3">
        <Button type="submit" variant={danger ? "danger" : "primary"} loading={busy}>
          {submitLabel}
        </Button>
        <Button type="button" variant="outline" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
