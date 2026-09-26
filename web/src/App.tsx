import {useEffect, useState, type FormEvent} from "react";
import {api, failure, type Site, type Viewer} from "./api/client";

type Mode = "login" | "register" | "reset";
const resetToken = new URLSearchParams(window.location.hash.slice(1)).get("reset") ?? "";
if (resetToken) window.history.replaceState(null, "", window.location.pathname);

async function loadDashboard() {
    const me = await api.GET("/api/auth/me");
    if (me.response.status === 401) return null;
    if (!me.data) throw failure(me.error);
    const result = await api.GET("/api/sites");
    if (!result.data) throw failure(result.error);
    return {viewer: me.data, sites: result.data};
}

export function App() {
    const [viewer, setViewer] = useState<Viewer | null>(null);
    const [sites, setSites] = useState<Site[]>([]);
    const [loading, setLoading] = useState(true);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");
    const [notice, setNotice] = useState("");
    const [mode, setMode] = useState<Mode>(resetToken ? "reset" : "login");

    async function refresh(requireSession = false) {
        const data = await loadDashboard();
        setViewer(data?.viewer ?? null);
        setSites(data?.sites ?? []);
        if (!data && requireSession) throw new Error("Your session could not be loaded. Sign in again using the configured dashboard address.");
    }

    useEffect(() => {
        let active = true;
        loadDashboard().then(data => {
            if (active) { setViewer(data?.viewer ?? null); setSites(data?.sites ?? []); }
        }).catch((err: unknown) => { if (active) setError(failure(err).message); })
            .finally(() => { if (active) setLoading(false); });
        return () => { active = false; };
    }, []);

    async function submit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        const values = new FormData(event.currentTarget);
        const email = String(values.get("email") ?? "");
        const password = String(values.get("password") ?? "");
        setBusy(true); setError(""); setNotice("");
        try {
            if (viewer && mode !== "reset") {
                const slug = String(values.get("slug"));
                const result = await api.POST("/api/sites", {
                    body: {slug}, headers: {"X-CSRF-Token": viewer.csrfToken},
                });
                if (result.error) throw failure(result.error);
            } else if (mode === "reset") {
                const result = await api.POST("/api/auth/reset-password", {body: {token: resetToken, password}});
                if (result.error) throw failure(result.error);
                setMode("login"); setViewer(null); setSites([]);
                setNotice("Password updated. Sign in with your new password.");
                return;
            } else if (mode === "register") {
                const invitation = String(values.get("invitation"));
                const result = await api.POST("/api/auth/register", {body: {email, password, invitation}});
                if (result.error) throw failure(result.error);
            } else {
                const result = await api.POST("/api/auth/login", {body: {email, password}});
                if (result.error) throw failure(result.error);
            }
            await refresh(true);
        } catch (err) { setError(failure(err).message); }
        finally { setBusy(false); }
    }

    async function logout() {
        setBusy(true); setError("");
        try {
            const result = await api.POST("/api/auth/logout", {headers: {"X-CSRF-Token": viewer?.csrfToken ?? ""}});
            if (result.error && result.response.status !== 401) throw failure(result.error);
            setViewer(null); setSites([]); setMode("login");
        } catch (err) { setError(failure(err).message); }
        finally { setBusy(false); }
    }

    return <main>
        <header><p className="brand">Lumio</p>{viewer && <button className="quiet" disabled={busy} onClick={logout}>Sign out</button>}</header>
        <section aria-labelledby="title">
            <p className="eyebrow">Your work, beautifully framed.</p>
            {loading ? <p role="status">Loading your dashboard…</p> : <>
                <h1 id="title">{mode === "reset" ? "A fresh start." : viewer ? "Your portfolio." : mode === "register" ? "Make yourself at home." : "Welcome back."}</h1>
                {error && <p role="alert" className="error">{error}</p>}
                {notice && <p role="status">{notice}</p>}
                {viewer && mode !== "reset" ? <>
                    <p className="intro">Signed in as {viewer.email}</p>
                    {sites.length ? <ul className="sites">{sites.map(site => <li key={site.id}>
                        <h2>{site.slug}.{viewer.baseDomain}</h2>
                        <span className="badge">Private draft</span>
                        <p>Your subdomain is reserved. Photo uploads and portfolio editing are coming next.</p>
                    </li>)}</ul> : <form onSubmit={submit}>
                        <p className="intro">Choose a home for your photography.</p>
                        <label htmlFor="slug">Your subdomain</label>
                        <div className="subdomain"><input id="slug" name="slug" required minLength={1} maxLength={63} pattern="[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*" autoCapitalize="none" placeholder="anna" /><span>.{viewer.baseDomain}</span></div>
                        <p className="hint">Letters, numbers and hyphens. System names are reserved.</p>
                        <button disabled={busy}>{busy ? "Creating…" : "Create portfolio"}</button>
                    </form>}
                </> : <>
                    <form key={mode} onSubmit={submit}>
                        {mode !== "reset" && <><label htmlFor="email">Email</label><input id="email" name="email" type="email" autoComplete="email" maxLength={254} required /></>}
                        <label htmlFor="password">{mode === "reset" ? "New password" : "Password"}</label>
                        <input id="password" name="password" type="password" autoComplete={mode === "login" ? "current-password" : "new-password"} minLength={12} maxLength={128} required />
                        <p className="hint">Use at least 12 characters (up to 128 bytes).</p>
                        {mode === "register" && <><label htmlFor="invitation">Invitation code</label><input id="invitation" name="invitation" autoComplete="off" minLength={26} maxLength={26} required /><p className="hint">Use the email address your invitation was issued to.</p></>}
                        <button disabled={busy}>{busy ? "Please wait…" : mode === "register" ? "Create account" : mode === "reset" ? "Set new password" : "Sign in"}</button>
                    </form>
                    {mode !== "reset" && <button className="quiet switch" disabled={busy} onClick={() => { setMode(mode === "login" ? "register" : "login"); setError(""); setNotice(""); }}>
                        {mode === "login" ? "Have an invitation? Create an account" : "Already have an account? Sign in"}
                    </button>}
                    {mode === "login" && <p className="hint">Forgot your password? Contact the person who invited you for a secure reset link.</p>}
                </>}
            </>}
        </section>
        <footer>Made for the way you see the world.</footer>
    </main>;
}
