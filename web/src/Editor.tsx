import {getLanguage, setLanguage, useLanguage, translate as t} from "./i18n";
import {useEffect, useState, type FormEvent} from "react";
import {Publishing} from "./Publishing";
import {api, failure} from "./api/client";
import type {components} from "./api/schema";

type Draft = components["schemas"]["Draft"];
type Photo = components["schemas"]["Photo"];
export function Editor({siteId, csrf}: {siteId: string; csrf: string}) {
    const language = useLanguage();
    const [draft, setDraft] = useState<Draft | null>(null);
    const [photos, setPhotos] = useState<Photo[]>([]);
    const [error, setError] = useState("");
    const [notice, setNotice] = useState("");
    const [busy, setBusy] = useState(false);
    const [edited, setDirty] = useState(false);
    const dirty = edited || (draft !== null && (draft.language || "en") !== language);
    const [reload, setReload] = useState(0);
    useEffect(() => {
        let active = true;
        const languageAtLoad = getLanguage();
        Promise.all([
            api.GET("/api/sites/{id}/draft", {params: {path: {id: siteId}}}),
            api.GET("/api/sites/{id}/photos", {params: {path: {id: siteId}}}),
        ]).then(([d, p]) => {
            if (!d.data) throw failure(d.error);
            if (!p.data) throw failure(p.error);
            if (active) {
                setDraft(d.data);
                if ((d.data.version > 0 || reload > 0) && getLanguage() === languageAtLoad) {
                    setLanguage(d.data.language || "en");
                }
                setPhotos(p.data); setDirty(false); setError("");
            }
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
            const result = await api.PUT("/api/sites/{id}/draft", {params: {path: {id: siteId}}, headers: {"X-CSRF-Token": csrf}, body: {...draft, language}});
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
    if (!draft) return <div>{error ? <><p role="alert">{t(error)}</p><button onClick={() => setReload(n => n + 1)}>{t("Retry loading editor")}</button></> : <p role="status">{t("Loading portfolio editor…")}</p>}</div>;
    const ready = photos.filter(p => p.status === "ready");
    const incomplete = [!draft.displayName.trim() && t("Add your display name."), !draft.biography.trim() && t("Introduce yourself with a biography."), !draft.photos.length && t("Select photographs for your portfolio."), !draft.contacts.length && t("Add a way for visitors to contact you."), draft.photos.some(p => !p.alt.trim()) && t("Add an image description to every photograph.")].filter(Boolean);
    const photoSelect = (field: "profilePhoto" | "coverPhoto", label: string) => <label>{t(label)}<select value={draft[field]} onChange={e => change({[field]: e.target.value})}><option value="">{t("None")}</option>{ready.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label>;
    return <div className="portfolio-editor">
        <h3>{t("Build your portfolio")}</h3>
        <p>{t("Save your draft, then preview it. Draft edits stay private until you publish.")}</p>
        <p className="hint">{t("Language changes apply to your portfolio when you save and publish.")}</p>
        <Publishing siteId={siteId} csrf={csrf} version={draft.version} dirty={dirty} saving={busy}/>
        {error && <p className="error" role="alert">{t(error)}</p>}
        {notice && <p role="status">{t(notice)}</p>}
        <form onSubmit={save}>
            <fieldset disabled={busy}>
                <legend>{t("About you")}</legend>
                <label>{t("Display name")}<input maxLength={120} value={draft.displayName} onChange={e => change({displayName: e.target.value})}/></label>
                <label>{t("Biography")}<textarea rows={5} maxLength={4000} value={draft.biography} onChange={e => change({biography: e.target.value})}/></label>
                <label>{t("Location")}<input maxLength={160} value={draft.location} onChange={e => change({location: e.target.value})}/></label>
                <label>{t("Specialization")}<input maxLength={160} placeholder={t("Portraits, weddings, travel…")} value={draft.specialization} onChange={e => change({specialization: e.target.value})}/></label>
                {photoSelect("profilePhoto", "Profile photograph")}
            </fieldset>
            <fieldset disabled={busy}>
                <legend>{t("Appearance")}</legend>
                <label>{t("Template")}<select value={draft.template} onChange={e => change({template: e.target.value as Draft["template"]})}><option value="gallery">{t("Gallery — centered introduction")}</option><option value="editorial">{t("Editorial — cover first, larger photographs")}</option></select></label>
                <label>{t("Typography")}<select value={draft.typography} onChange={e => change({typography: e.target.value as Draft["typography"]})}><option value="serif">{t("Classic serif")}</option><option value="sans">{t("Modern sans serif")}</option></select></label>
                <label>{t("Colour scheme")}<select value={draft.colour} onChange={e => change({colour: e.target.value as Draft["colour"]})}><option value="light">{t("Light")}</option><option value="dark">{t("Dark")}</option><option value="warm">{t("Warm")}</option></select></label>
                <label>{t("Photo layout")}<select value={draft.layout} onChange={e => change({layout: e.target.value as Draft["layout"]})}><option value="grid">{t("Grid")}</option><option value="column">{t("Single column")}</option></select></label>
                {photoSelect("coverPhoto", "Cover photograph")}
            </fieldset>
            <fieldset disabled={busy}>
                <legend>{t("Portfolio photographs")}</legend>
                <button type="button" className="quiet" onClick={() => void refreshPhotos()}>{t("Refresh uploaded photographs")}</button>
                {!ready.length && <p>{t("Upload photographs below. They can be selected once processing finishes.")}</p>}
                <label>{t("Add photograph")}<select value="" disabled={draft.photos.length >= 100} onChange={e => {if (e.target.value) change({photos: [...draft.photos, {id: e.target.value, alt: "", category: ""}]});}}><option value="">{t("Choose a processed photograph")}</option>{ready.filter(p => !draft.photos.some(selected => selected.id === p.id)).map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
                {!draft.photos.length && <p>{t("No photographs selected yet.")}</p>}
                <ol className="selected-photos">{draft.photos.map((p, i) => <li key={p.id}>
                    <strong>{photos.find(photo => photo.id === p.id)?.name ?? t("Unavailable photograph")}</strong>
                    <label>{t("Image description")}<input maxLength={300} value={p.alt} onChange={e => change({photos: draft.photos.map((item, n) => n === i ? {...item, alt: e.target.value} : item)})}/></label>
                    <label>{t("Category (optional)")}<input maxLength={80} placeholder={t("Portraits")} value={p.category} onChange={e => change({photos: draft.photos.map((item, n) => n === i ? {...item, category: e.target.value} : item)})}/></label>
                    <div className="editor-actions"><button type="button" className="secondary" disabled={i === 0} onClick={() => move(i, -1)}>{t("Move up")}</button><button type="button" className="secondary" disabled={i === draft.photos.length - 1} onClick={() => move(i, 1)}>{t("Move down")}</button><button type="button" className="danger" onClick={() => change({photos: draft.photos.filter((_, n) => n !== i)})}>{t("Deselect")}</button></div>
                </li>)}</ol>
            </fieldset>
            <fieldset disabled={busy}>
                <legend>{t("Services and pricing")}</legend>
                <label className="check"><input type="checkbox" checked={draft.hidePrices} onChange={e => change({hidePrices: e.target.checked})}/>{t("Hide prices")}</label>
                {draft.services.map((service, i) => <div className="editor-row" key={i}>
                    <label>{t("Service name")}<input required maxLength={120} value={service.name} onChange={e => change({services: draft.services.map((s, n) => n === i ? {...s, name: e.target.value} : s)})}/></label>
                    <label>{t("Description")}<textarea maxLength={1000} value={service.description} onChange={e => change({services: draft.services.map((s, n) => n === i ? {...s, description: e.target.value} : s)})}/></label>
                    <label>{t("Price")}<input maxLength={100} placeholder={t("From €250")} value={service.price} onChange={e => change({services: draft.services.map((s, n) => n === i ? {...s, price: e.target.value} : s)})}/></label>
                    <button type="button" className="danger" onClick={() => change({services: draft.services.filter((_, n) => n !== i)})}>{t("Remove service")}</button>
                </div>)}
                <button type="button" className="secondary" disabled={draft.services.length >= 20} onClick={() => change({services: [...draft.services, {name: "", description: "", price: ""}]})}>{t("Add service")}</button>
            </fieldset>
            <fieldset disabled={busy}>
                <legend>{t("Contact links")}</legend>
                <p className="hint">{t("Use mailto:hello@example.com for email, tel:+123456789 for telephone, https://t.me/username for Telegram, https://wa.me/123456789 for WhatsApp, or another HTTPS address.")}</p>
                {draft.contacts.map((contact, i) => <div className="editor-row" key={i}>
                    <label>{t("Link label")}<input required maxLength={60} placeholder={t("Email / Telegram / WhatsApp")} value={contact.label} onChange={e => change({contacts: draft.contacts.map((c, n) => n === i ? {...c, label: e.target.value} : c)})}/></label>
                    <label>{t("Contact address")}<input required maxLength={500} placeholder="mailto:hello@example.com" value={contact.url} onChange={e => change({contacts: draft.contacts.map((c, n) => n === i ? {...c, url: e.target.value} : c)})}/></label>
                    <button type="button" className="danger" onClick={() => change({contacts: draft.contacts.filter((_, n) => n !== i)})}>{t("Remove contact")}</button>
                </div>)}
                <button type="button" className="secondary" disabled={draft.contacts.length >= 12} onClick={() => change({contacts: [...draft.contacts, {label: "", url: ""}]})}>{t("Add contact")}</button>
            </fieldset>
            <fieldset disabled={busy}>
                <legend>{t("Search and sharing")}</legend>
                <label>{t("Page title")}<input maxLength={120} placeholder={`${draft.displayName || t("Your name")} — ${t("Photography")}`} value={draft.seoTitle ?? ""} onChange={e => change({seoTitle: e.target.value})}/></label>
                <label>{t("Page description")}<textarea maxLength={300} placeholder={t("A short introduction for search results and shared links")} value={draft.seoDescription ?? ""} onChange={e => change({seoDescription: e.target.value})}/></label>
                <p className="hint">{t("Leave these blank to use your name and biography. Your cover photograph, or first portfolio photograph, appears in link previews.")}</p>
            </fieldset>
            {!!incomplete.length && <aside><p>{t("Before sharing your portfolio:")}</p><ul>{incomplete.map(item => <li key={String(item)}>{item}</li>)}</ul><p>{t("You can save an incomplete draft.")}</p></aside>}
            <div className="editor-actions"><button disabled={busy}>{busy ? t("Please wait…") : t("Save draft")}</button><span role="status">{dirty ? t("Unsaved changes") : t("Saved draft")}</span></div>
        </form>
        <div className="editor-actions"><a href={`/preview/${siteId}`} target="_blank" rel="noreferrer">{t("Preview saved portfolio ↗")}</a><button type="button" className="quiet" disabled={busy} onClick={() => {if (!dirty || window.confirm(t("Discard unsaved changes and reload the saved draft?"))) {setBusy(true); setReload(n => n + 1);}}}>{t("Reload saved draft")}</button></div>
        {dirty && <p className="hint">{t("Save first to include your latest changes in the preview.")}</p>}
    </div>;
}
