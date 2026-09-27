(() => {
    let shown = false;
    function recover() {
        if (shown) return;
        shown = true;
        const show = () => {
            const notice = document.createElement("aside");
            notice.setAttribute("role", "alert");
            notice.textContent = "Lumio has been updated or could not finish loading. Save any work you can, then ";
            const reload = document.createElement("button");
            reload.textContent = "Reload Lumio";
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
