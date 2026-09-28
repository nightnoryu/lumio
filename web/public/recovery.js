(() => {
    let shown = false;
    function recover() {
        if (shown) return;
        shown = true;
        const show = () => {
            let russian = document.documentElement.lang === "ru";
            try { russian = window.localStorage.getItem("lumio.language") === "ru"; } catch { /* Use the document language. */ }
            const notice = document.createElement("aside");
            notice.setAttribute("role", "alert");
            notice.textContent = russian ? "Lumio обновлён или не смог загрузиться. Сохраните доступные изменения, затем " : "Lumio has been updated or could not finish loading. Save any work you can, then ";
            const reload = document.createElement("button");
            reload.textContent = russian ? "Перезагрузить Lumio" : "Reload Lumio";
            reload.onclick = () => window.location.reload();
            notice.append(reload);
            document.body.prepend(notice);
        };
        if (document.body) show();
        else document.addEventListener("DOMContentLoaded", show, {once: true});
    }
    window.addEventListener("error", event => {
        const target = event.target;
        if ((target instanceof HTMLScriptElement || target instanceof HTMLLinkElement) &&
            new URL(target.src || target.href, location.href).pathname.startsWith("/assets/")) recover();
    }, true);
    window.addEventListener("vite:preloadError", event => {
        event.preventDefault();
        recover();
    });
})();
