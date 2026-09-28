import {useLanguage, translate as t} from "./i18n";
import {useEffect, useState} from "react";
import {api, failure} from "./api/client";
import type {components} from "./api/schema";

type Publication = components["schemas"]["Publication"];
export function Publishing({siteId, csrf, version, dirty, saving}: {siteId: string; csrf: string; version: number; dirty: boolean; saving: boolean}) {
    useLanguage();
    const [publication, setPublication] = useState<Publication | null>(null);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");
    const [notice, setNotice] = useState("");
    const [reload, setReload] = useState(0);
    useEffect(() => {
        let active = true;
        api.GET("/api/sites/{id}/publication", {params: {path: {id: siteId}}}).then(result => {
            if (!result.data) throw failure(result.error);
            if (active) {setPublication(result.data); setError("");}
        }).catch((err: unknown) => {if (active) setError(failure(err).message);});
        return () => {active = false;};
    }, [siteId, reload]);
    async function publish(remove: boolean) {
        setBusy(true); setError(""); setNotice("");
        const options = {params: {path: {id: siteId}}, headers: {"X-CSRF-Token": csrf, "Content-Type": "application/json"}};
        try {
            if (remove) {
                const result = await api.DELETE("/api/sites/{id}/publication", options);
                if (result.error) throw failure(result.error);
                setPublication(p => p ? {...p, published: false, version: 0} : p);
                setNotice("Portfolio unpublished. Visitors can no longer open it.");
            } else {
                const result = await api.POST("/api/sites/{id}/publication", {...options, body: {version}});
                if (!result.data) throw failure(result.error);
                setPublication(result.data);
                setNotice("Your portfolio is now published.");
            }
        } catch (err) {setError(failure(err).message);} finally {setBusy(false);}
    }
    return <section className="publishing" aria-label={t("Publishing")}>
        {error && <p className="error" role="alert">{t(error)}</p>}
        {notice && <p role="status">{t(notice)}</p>}
        {!publication ? error ? <button onClick={() => setReload(n => n + 1)}>{t("Retry publication status")}</button> : <p role="status">{t("Loading publication status…")}</p> : <>
            <p className="badge">{publication.published ? publication.version === version && !dirty ? t("Published · up to date") : t("Published · unpublished changes") : t("Private portfolio")}</p>
            {publication.published && <p><a href={publication.url} target="_blank" rel="noreferrer">{t("Open live portfolio ↗")}</a></p>}
            <div className="editor-actions">
                <button type="button" disabled={busy || saving || dirty || version < 1} onClick={() => void publish(false)}>{busy ? t("Please wait…") : publication.published ? t("Publish saved changes") : t("Publish portfolio")}</button>
                {publication.published && <button type="button" className="quiet" disabled={busy || saving} onClick={() => void publish(true)}>{t("Unpublish portfolio")}</button>}
            </div>
            {(dirty || version < 1) && <p className="hint">{t("Save your draft before publishing. Preview it to check the layout.")}</p>}
        </>}
    </section>;
}
