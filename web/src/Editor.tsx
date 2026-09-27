import {useEffect, useState, type FormEvent} from "react";
import {Publishing} from "./Publishing";
import {api, failure} from "./api/client";
import type {components} from "./api/schema";

type Draft = components["schemas"]["Draft"];
type Photo = components["schemas"]["Photo"];
export function Editor({siteId, csrf}: {siteId: string; csrf: string}) {
    const [draft, setDraft] = useState<Draft | null>(null);
    const [photos, setPhotos] = useState<Photo[]>([]);
    const [error, setError] = useState("");
    const [notice, setNotice] = useState("");
    const [busy, setBusy] = useState(false);
    const [dirty, setDirty] = useState(false);
    const [reload, setReload] = useState(0);
    useEffect(() => {
        let active = true;
        Promise.all([
            api.GET("/api/sites/{id}/draft", {params: {path: {id: siteId}}}),
            api.GET("/api/sites/{id}/photos", {params: {path: {id: siteId}}}),
        ]).then(([d, p]) => {
            if (!d.data) throw failure(d.error);
            if (!p.data) throw failure(p.error);
            if (active) {setDraft(d.data); setPhotos(p.data); setDirty(false); setError("");}
        }).catch((err: unknown) => {if (active) setError(failure(err).message);})
            .finally(() => {if (active) setBusy(false);});
        return () => {active = false;};
    }, [siteId, reload]);
    useEffect(() => {
        if (!dirty) return;
        const warn = (event: BeforeUnloadEvent) => {event.preventDefault();};
        window.addEventListener("beforeunload", warn);
        return () => window.removeEventListener("beforeunload", warn);
    }, [dirty]);
    function change(patch: Partial<Draft>) {setDraft(d => d ? {...d, ...patch} : d); setDirty(true); setNotice("");}
    async function save(event: FormEvent) {
        event.preventDefault(); if (!draft) return;
        setBusy(true); setError(""); setNotice("");
        try {
            const result = await api.PUT("/api/sites/{id}/draft", {params: {path: {id: siteId}}, headers: {"X-CSRF-Token": csrf}, body: draft});
            if (!result.data) throw failure(result.error);
            setDraft(result.data); setDirty(false); setNotice("Draft saved. Publish to make these changes public.");
        } catch (err) {setError(failure(err).message);} finally {setBusy(false);}
    }
    async function refreshPhotos() {
        setError("");
        try {const result = await api.GET("/api/sites/{id}/photos", {params: {path: {id: siteId}}}); if (!result.data) throw failure(result.error); setPhotos(result.data);}
        catch (err) {setError(failure(err).message);}
    }
    function move(index: number, offset: number) {
        if (!draft) return;
        const next = [...draft.photos];
        [next[index], next[index + offset]] = [next[index + offset], next[index]];
        change({photos: next});
    }
    if (!draft) return <div>{error ? <><p role="alert">{error}</p><button onClick={() => setReload(n => n + 1)}>Retry loading editor</button></> : <p role="status">Loading portfolio editor…</p>}</div>;
    const ready = photos.filter(p => p.status === "ready");
    const incomplete = [!draft.displayName.trim() && "Add your display name.", !draft.biography.trim() && "Introduce yourself with a biography.", !draft.photos.length && "Select photographs for your portfolio.", !draft.contacts.length && "Add a way for visitors to contact you.", draft.photos.some(p => !p.alt.trim()) && "Add an image description to every photograph."].filter(Boolean);
    const photoSelect = (field: "profilePhoto" | "coverPhoto", label: string) => <label>{label}<select value={draft[field]} onChange={e => change({[field]: e.target.value})}><option value="">None</option>{ready.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label>;
    return <div className="portfolio-editor">
        <h3>Build your portfolio</h3>
        <p>Save your draft, then preview it. Draft edits stay private until you publish.</p>
        <Publishing siteId={siteId} csrf={csrf} version={draft.version} dirty={dirty} saving={busy}/>
        {error && <p className="error" role="alert">{error}</p>}
        {notice && <p role="status">{notice}</p>}
        <form onSubmit={save}>
            <fieldset disabled={busy}>
                <legend>About you</legend>
                <label>Display name<input maxLength={120} value={draft.displayName} onChange={e => change({displayName: e.target.value})}/></label>
                <label>Biography<textarea rows={5} maxLength={4000} value={draft.biography} onChange={e => change({biography: e.target.value})}/></label>
                <label>Location<input maxLength={160} value={draft.location} onChange={e => change({location: e.target.value})}/></label>
                <label>Specialization<input maxLength={160} placeholder="Portraits, weddings, travel…" value={draft.specialization} onChange={e => change({specialization: e.target.value})}/></label>
                {photoSelect("profilePhoto", "Profile photograph")}
            </fieldset>
            <fieldset disabled={busy}>
                <legend>Appearance</legend>
                <label>Template<select value={draft.template} onChange={e => change({template: e.target.value as Draft["template"]})}><option value="gallery">Gallery — centered introduction</option><option value="editorial">Editorial — cover first, larger photographs</option></select></label>
                <label>Typography<select value={draft.typography} onChange={e => change({typography: e.target.value as Draft["typography"]})}><option value="serif">Classic serif</option><option value="sans">Modern sans serif</option></select></label>
                <label>Colour scheme<select value={draft.colour} onChange={e => change({colour: e.target.value as Draft["colour"]})}><option value="light">Light</option><option value="dark">Dark</option><option value="warm">Warm</option></select></label>
                <label>Photo layout<select value={draft.layout} onChange={e => change({layout: e.target.value as Draft["layout"]})}><option value="grid">Grid</option><option value="column">Single column</option></select></label>
                {photoSelect("coverPhoto", "Cover photograph")}
            </fieldset>
            <fieldset disabled={busy}>
                <legend>Portfolio photographs</legend>
                <button type="button" className="quiet" onClick={() => void refreshPhotos()}>Refresh uploaded photographs</button>
                {!ready.length && <p>Upload photographs below. They can be selected once processing finishes.</p>}
                <label>Add photograph<select value="" disabled={draft.photos.length >= 100} onChange={e => {if (e.target.value) change({photos: [...draft.photos, {id: e.target.value, alt: "", category: ""}]});}}><option value="">Choose a processed photograph</option>{ready.filter(p => !draft.photos.some(selected => selected.id === p.id)).map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
                {!draft.photos.length && <p>No photographs selected yet.</p>}
                <ol className="selected-photos">{draft.photos.map((p, i) => <li key={p.id}>
                    <strong>{photos.find(photo => photo.id === p.id)?.name ?? "Unavailable photograph"}</strong>
                    <label>Image description<input maxLength={300} value={p.alt} onChange={e => change({photos: draft.photos.map((item, n) => n === i ? {...item, alt: e.target.value} : item)})}/></label>
                    <label>Category (optional)<input maxLength={80} placeholder="Portraits" value={p.category} onChange={e => change({photos: draft.photos.map((item, n) => n === i ? {...item, category: e.target.value} : item)})}/></label>
                    <div className="editor-actions"><button type="button" disabled={i === 0} onClick={() => move(i, -1)}>Move up</button><button type="button" disabled={i === draft.photos.length - 1} onClick={() => move(i, 1)}>Move down</button><button type="button" onClick={() => change({photos: draft.photos.filter((_, n) => n !== i)})}>Deselect</button></div>
                </li>)}</ol>
            </fieldset>
            <fieldset disabled={busy}>
                <legend>Services and pricing</legend>
                <label className="check"><input type="checkbox" checked={draft.hidePrices} onChange={e => change({hidePrices: e.target.checked})}/>Hide prices</label>
                {draft.services.map((service, i) => <div className="editor-row" key={i}>
                    <label>Service name<input required maxLength={120} value={service.name} onChange={e => change({services: draft.services.map((s, n) => n === i ? {...s, name: e.target.value} : s)})}/></label>
                    <label>Description<textarea maxLength={1000} value={service.description} onChange={e => change({services: draft.services.map((s, n) => n === i ? {...s, description: e.target.value} : s)})}/></label>
                    <label>Price<input maxLength={100} placeholder="From €250" value={service.price} onChange={e => change({services: draft.services.map((s, n) => n === i ? {...s, price: e.target.value} : s)})}/></label>
                    <button type="button" className="quiet" onClick={() => change({services: draft.services.filter((_, n) => n !== i)})}>Remove service</button>
                </div>)}
                <button type="button" disabled={draft.services.length >= 20} onClick={() => change({services: [...draft.services, {name: "", description: "", price: ""}]})}>Add service</button>
            </fieldset>
            <fieldset disabled={busy}>
                <legend>Contact links</legend>
                <p className="hint">Use mailto:hello@example.com for email, tel:+123456789 for telephone, https://t.me/username for Telegram, https://wa.me/123456789 for WhatsApp, or another HTTPS address.</p>
                {draft.contacts.map((contact, i) => <div className="editor-row" key={i}>
                    <label>Link label<input required maxLength={60} placeholder="Email / Telegram / WhatsApp" value={contact.label} onChange={e => change({contacts: draft.contacts.map((c, n) => n === i ? {...c, label: e.target.value} : c)})}/></label>
                    <label>Contact address<input required maxLength={500} placeholder="mailto:hello@example.com" value={contact.url} onChange={e => change({contacts: draft.contacts.map((c, n) => n === i ? {...c, url: e.target.value} : c)})}/></label>
                    <button type="button" className="quiet" onClick={() => change({contacts: draft.contacts.filter((_, n) => n !== i)})}>Remove contact</button>
                </div>)}
                <button type="button" disabled={draft.contacts.length >= 12} onClick={() => change({contacts: [...draft.contacts, {label: "", url: ""}]})}>Add contact</button>
            </fieldset>
            <fieldset disabled={busy}>
                <legend>Search and sharing</legend>
                <label>Page title<input maxLength={120} placeholder={`${draft.displayName || "Your name"} — Photography`} value={draft.seoTitle ?? ""} onChange={e => change({seoTitle: e.target.value})}/></label>
                <label>Page description<textarea maxLength={300} placeholder="A short introduction for search results and shared links" value={draft.seoDescription ?? ""} onChange={e => change({seoDescription: e.target.value})}/></label>
                <p className="hint">Leave these blank to use your name and biography. Your cover photograph, or first portfolio photograph, appears in link previews.</p>
            </fieldset>
            {!!incomplete.length && <aside><p>Before sharing your portfolio:</p><ul>{incomplete.map(item => <li key={String(item)}>{item}</li>)}</ul><p>You can save an incomplete draft.</p></aside>}
            <div className="editor-actions"><button disabled={busy}>{busy ? "Please wait…" : "Save draft"}</button><span role="status">{dirty ? "Unsaved changes" : "Saved draft"}</span></div>
        </form>
        <div className="editor-actions"><a href={`/preview/${siteId}`} target="_blank" rel="noreferrer">Preview saved portfolio ↗</a><button type="button" className="quiet" disabled={busy} onClick={() => {if (!dirty || window.confirm("Discard unsaved changes and reload the saved draft?")) {setBusy(true); setReload(n => n + 1);}}}>Reload saved draft</button></div>
        {dirty && <p className="hint">Save first to include your latest changes in the preview.</p>}
    </div>;
}
