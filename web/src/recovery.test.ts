// @vitest-environment jsdom
import {afterEach, expect, test, vi} from "vitest";
import source from "../public/recovery.js?raw";

afterEach(() => { document.body.innerHTML = ""; vi.restoreAllMocks(); });

test("missing previous-deployment bundles offer recovery without discarding edits", () => {
    const listeners = new Map<string, EventListener>();
    vi.spyOn(window, "addEventListener").mockImplementation((name, callback) => {
        listeners.set(name, callback as EventListener);
    });
    new Function(source)();
    document.body.innerHTML = `<input value="unsaved biography">`;
    const script = document.createElement("script");
    script.src = "/assets/previous-12345678.js";
    const error = new Event("error");
    Object.defineProperty(error, "target", {value: script});
    listeners.get("error")!(error);
    expect(document.querySelector("[role=alert]")?.textContent).toContain("Reload Lumio");
    expect(document.querySelector("input")?.value).toBe("unsaved biography");
    const preload = new Event("vite:preloadError", {cancelable: true});
    listeners.get("vite:preloadError")!(preload);
    expect(preload.defaultPrevented).toBe(true);
    expect(document.querySelectorAll("[role=alert]")).toHaveLength(1);
});
