import {useLanguage, translate as t} from "./i18n";
import {useEffect, useRef, useState} from "react";
import Uppy from "@uppy/core";
import Dashboard from "@uppy/dashboard";
import ru from "@uppy/locales/lib/ru_RU";
import en from "@uppy/locales/lib/en_US";
import AwsS3 from "@uppy/aws-s3";
import "@uppy/core/css/style.min.css";
import "@uppy/dashboard/css/style.min.css";
import {api, failure} from "./api/client";
import type {components} from "./api/schema";

type Photo = components["schemas"]["Photo"];
export function Photos({siteId, csrf}: {siteId: string; csrf: string}) {
    const language = useLanguage();
    const uploader = useRef<Uppy | null>(null);
    const target = useRef<HTMLDivElement>(null);
    const previewCache = useRef<Record<string, {url: string; expires: number}>>({});
    const watermark = useRef<HTMLInputElement>(null);
    const [photos, setPhotos] = useState<Photo[]>([]);
    const [previews, setPreviews] = useState<Record<string, string>>({});
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    const [revision, setRevision] = useState(0);
    useEffect(() => {
        let active = true;
        async function refresh() {
            const result = await api.GET("/api/sites/{id}/photos", {params: {path: {id: siteId}}});
            if (!result.data) throw failure(result.error);
            if (!active) return;
            setPhotos(result.data); setLoading(false);
            const images: Record<string, string> = {};
            await Promise.all(result.data.filter(p => p.status === "ready").map(async p => {
                const cached = previewCache.current[p.id];
                if (cached && cached.expires > Date.now()) {images[p.id] = cached.url; return;}
                const preview = await api.GET("/api/sites/{id}/photos/{photoId}/preview", {params: {path: {id: siteId, photoId: p.id}}});
                if (preview.data) {
                    images[p.id] = preview.data.url;
                    previewCache.current[p.id] = {url: preview.data.url, expires: Date.now() + 240_000};
                }
            }));
            if (active) setPreviews(images);
        }
        const poll = () => { void refresh().catch((err: unknown) => {if (active) {setError(failure(err).message); setLoading(false);}}); };
        poll();
        const timer = window.setInterval(poll, 5000);
        return () => {active = false; window.clearInterval(timer);};
    }, [siteId, revision]);
    useEffect(() => {
        if (!target.current) return;
        const uploads = new Map<string, {id: string; url: string}>();
        const uppy = new Uppy({restrictions: {allowedFileTypes: ["image/jpeg", "image/png", "image/webp"]}, autoProceed: false})
            .use(Dashboard, {target: target.current, inline: true, height: 360, proudlyDisplayPoweredByUppy: false, hideProgressDetails: false})
            .use(AwsS3, {shouldUseMultipart: false, async getUploadParameters(file) {
                let upload = uploads.get(file.id);
                if (!upload) {
                    const result = await api.POST("/api/sites/{id}/photos", {params: {path: {id: siteId}}, headers: {"X-CSRF-Token": csrf}, body: {
                        name: file.name ?? t("Photograph"), size: file.size ?? 0, contentType: file.type as "image/jpeg" | "image/png" | "image/webp", watermark: watermark.current?.value ?? "",
                    }});
                    if (!result.data) throw failure(result.error);
                    upload = result.data; uploads.set(file.id, upload);
                    setRevision(n => n + 1);
                }
                return {method: "PUT", url: upload.url, headers: {"Content-Type": file.type, "If-None-Match": "*"}};
            }});
        uploader.current = uppy;
        async function complete(fileId: string) {
            const upload = uploads.get(fileId); if (!upload) return;
            const result = await api.POST("/api/sites/{id}/photos/{photoId}/complete", {params: {path: {id: siteId, photoId: upload.id}}, headers: {"X-CSRF-Token": csrf, "Content-Type": "application/json"}});
            if (result.error) throw failure(result.error);
            setRevision(n => n + 1);
        }
        uppy.on("upload-success", file => {if (file) void complete(file.id).catch((err: unknown) => setError(failure(err).message));});
        uppy.on("file-removed", file => {
            const upload = uploads.get(file.id);
            if (!upload || file.progress.uploadComplete) return;
            void api.DELETE("/api/sites/{id}/photos/{photoId}", {params: {path: {id: siteId, photoId: upload.id}}, headers: {"X-CSRF-Token": csrf, "Content-Type": "application/json"}})
                .then(result => {if (result.error) throw failure(result.error); setRevision(n => n + 1);}).catch((err: unknown) => setError(failure(err).message));
        });
        return () => {uploader.current = null; uppy.destroy();};
    }, [siteId, csrf]);
    useEffect(() => {
        uploader.current?.setOptions({locale: language === "ru" ? ru : en});
    }, [language, siteId, csrf]);
    async function action(photo: Photo, remove: boolean) {
        setError("");
        try {
            const options = {params: {path: {id: siteId, photoId: photo.id}}, headers: {"X-CSRF-Token": csrf, "Content-Type": "application/json"}};
            const result = remove ? await api.DELETE("/api/sites/{id}/photos/{photoId}", options) : await api.POST("/api/sites/{id}/photos/{photoId}/complete", options);
            if (result.error) throw failure(result.error);
            setRevision(n => n + 1);
        } catch (err) {setError(failure(err).message);}
    }
    return <div className="photo-manager">
        <h3>{t("Your photographs")}</h3>
        <p>{t("Upload JPEG, PNG or WebP. Originals stay private. Failed transfers can be retried in the uploader.")}</p>
        <label>{t("Watermark for new uploads (optional)")}<input ref={watermark} maxLength={80} placeholder={t("Your name")} aria-label={t("Watermark")} /></label>
        <div ref={target} />
        {error && <p role="alert" className="error">{t(error)}</p>}
        {loading && <p role="status">{t("Loading photographs…")}</p>}
        {!loading && !error && !photos.length && <p>{t("No photographs yet.")}</p>}
        <ul className="photo-grid">{photos.map(photo => <li key={photo.id}>
            {previews[photo.id] && <img src={previews[photo.id]} alt={photo.name} />}
            <strong>{photo.name}</strong><span>{t(photo.status)}</span>
            {photo.status === "failed" && <p>{t("Processing failed. Remove this photo and try uploading a valid image again.")}</p>}
            {photo.status === "uploading" && <button className="quiet" onClick={() => void action(photo, false)}>{t("Check completed upload")}</button>}
            <button className="danger" onClick={() => void action(photo, true)}>{t("Remove")}</button>
        </li>)}</ul>
    </div>;
}
