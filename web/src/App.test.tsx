vi.mock("./Editor", () => ({Editor: () => <div>Portfolio editor</div>}));
vi.mock("./Photos", () => ({Photos: () => <div>Photographs</div>}));
// @vitest-environment jsdom
import {fireEvent, render, screen, waitFor, cleanup} from "@testing-library/react";
import {afterEach, beforeEach, expect, test, vi} from "vitest";
import {setLanguage} from "./i18n";
import {App} from "./App";
import {api} from "./api/client";

vi.mock("./api/client", () => ({
    api: {GET: vi.fn(), POST: vi.fn()},
    failure: (error: unknown) => error instanceof Error ? error : new Error("Request failed"),
}));

const viewer = {id: "owner", email: "anna@example.com", csrfToken: "csrf-secret", baseDomain: "example.com"};
beforeEach(() => vi.resetAllMocks());
afterEach(() => {cleanup(); setLanguage("en"); vi.unstubAllGlobals();});

test("sign in, reserve a subdomain, and sign out", async () => {
    const get = vi.mocked(api.GET);
    get.mockResolvedValueOnce({response: new Response(null, {status: 401}), error: {message: "Sign in required"}});
    render(<App />);
    await screen.findByRole("heading", {name: "Welcome back."});
    get.mockResolvedValueOnce({response: new Response(), data: viewer});
    get.mockResolvedValueOnce({response: new Response(), data: []});
    vi.mocked(api.POST).mockResolvedValue({response: new Response(null, {status: 204})});
    fireEvent.change(screen.getByLabelText("Email"), {target: {value: viewer.email}});
    fireEvent.change(screen.getByLabelText("Password"), {target: {value: "long-enough-password"}});
    fireEvent.click(screen.getByRole("button", {name: "Sign in"}));
    await screen.findByLabelText("Your subdomain");
    get.mockResolvedValueOnce({response: new Response(), data: viewer});
    get.mockResolvedValueOnce({response: new Response(), data: [{id: "site", slug: "anna", createdAt: "2026-09-26T00:00:00Z"}]});
    fireEvent.change(screen.getByLabelText("Your subdomain"), {target: {value: "anna"}});
    fireEvent.click(screen.getByRole("button", {name: "Create portfolio"}));
    await screen.findByRole("heading", {name: "anna.example.com"});
    expect(api.POST).toHaveBeenCalledWith("/api/sites", {body: {slug: "anna"}, headers: {"X-CSRF-Token": viewer.csrfToken}});
    expect(await screen.findByText("Portfolio editor")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", {name: "Sign out"}));
    await screen.findByRole("heading", {name: "Welcome back."});
    expect(api.POST).toHaveBeenCalledWith("/api/auth/logout", {headers: {"X-CSRF-Token": viewer.csrfToken}});
});

test("shows authentication failures and allows retry", async () => {
    vi.mocked(api.GET).mockResolvedValue({response: new Response(null, {status: 401}), error: {message: "Sign in required"}});
    vi.mocked(api.POST).mockRejectedValue(new Error("Connection unavailable"));
    render(<App />);
    await screen.findByRole("heading", {name: "Welcome back."});
    fireEvent.change(screen.getByLabelText("Email"), {target: {value: viewer.email}});
    fireEvent.change(screen.getByLabelText("Password"), {target: {value: "long-enough-password"}});
    fireEvent.click(screen.getByRole("button", {name: "Sign in"}));
    await waitFor(() => expect(screen.getByRole("alert").textContent).toBe("Connection unavailable"));
    expect((screen.getByRole("button", {name: "Sign in"}) as HTMLButtonElement).disabled).toBe(false);
});


test("English is the default and the language selector persists Russian without clearing input", async () => {
    const stored = new Map<string, string>();
    vi.stubGlobal("localStorage", {
        getItem: (key: string) => stored.get(key) ?? null,
        setItem: (key: string, value: string) => {stored.set(key, value);},
    });
    vi.mocked(api.GET).mockResolvedValue({response: new Response(null, {status: 401}), error: {message: "Sign in required"}});
    const {unmount} = render(<App />);
    await screen.findByRole("heading", {name: "Welcome back."});
    expect(document.documentElement.lang).toBe("en");
    fireEvent.change(screen.getByLabelText("Email"), {target: {value: viewer.email}});
    fireEvent.change(screen.getByLabelText("Language"), {target: {value: "ru"}});
    expect(screen.getByRole("heading", {name: "С возвращением."})).toBeTruthy();
    expect((screen.getByLabelText("Электронная почта") as HTMLInputElement).value).toBe(viewer.email);
    expect(document.documentElement.lang).toBe("ru");
    expect(window.localStorage.getItem("lumio.language")).toBe("ru");
    unmount();
    render(<App />);
    await screen.findByRole("heading", {name: "С возвращением."});
    fireEvent.change(screen.getByLabelText("Язык"), {target: {value: "en"}});
    expect(screen.getByRole("heading", {name: "Welcome back."})).toBeTruthy();
});
