// @vitest-environment jsdom
import {act, fireEvent, render, screen, cleanup} from "@testing-library/react";
import {afterEach, expect, test, vi} from "vitest";
import {setLanguage} from "./i18n";
import {Editor} from "./Editor";
import {api} from "./api/client";
vi.mock("./Publishing", () => ({Publishing: () => <div>Publishing controls</div>}));
vi.mock("./api/client", () => ({api: {GET: vi.fn(), PUT: vi.fn()}, failure: (error: unknown) => error instanceof Error ? error : new Error("Request failed")}));
afterEach(() => {cleanup(); setLanguage("en"); vi.resetAllMocks();});
const draft = {language: "en" as const, version: 0, displayName: "", biography: "", location: "", specialization: "", profilePhoto: "", coverPhoto: "", template: "gallery" as const, typography: "serif" as const, colour: "light" as const, layout: "grid" as const, hidePrices: false, photos: [], services: [], contacts: []};
test("create a complete private portfolio through the form and save with CSRF", async () => {
    vi.mocked(api.GET).mockResolvedValueOnce({response: new Response(), data: draft});
    vi.mocked(api.GET).mockResolvedValueOnce({response: new Response(), data: [{id: "photo", name: "Portrait.jpg", status: "ready", width: 1200, height: 800, size: 100}]});
    vi.mocked(api.PUT).mockResolvedValue({response: new Response(), data: {...draft, version: 1, displayName: "Anna"}});
    render(<Editor siteId="site" csrf="csrf"/>);
    await screen.findByLabelText("Display name");
    fireEvent.change(screen.getByLabelText("Display name"), {target: {value: "Anna"}});
    fireEvent.change(screen.getByLabelText("Biography"), {target: {value: "Portrait photographer"}});
    fireEvent.change(screen.getByLabelText("Template"), {target: {value: "editorial"}});
    fireEvent.change(screen.getByLabelText("Add photograph"), {target: {value: "photo"}});
    fireEvent.change(screen.getByLabelText("Image description"), {target: {value: "A portrait in sunlight"}});
    fireEvent.click(screen.getByRole("button", {name: "Add service"}));
    fireEvent.change(screen.getByLabelText("Service name"), {target: {value: "Portrait session"}});
    fireEvent.change(screen.getByLabelText("Price"), {target: {value: "€250"}});
    fireEvent.click(screen.getByLabelText("Hide prices"));
    fireEvent.click(screen.getByRole("button", {name: "Add contact"}));
    fireEvent.change(screen.getByLabelText("Link label"), {target: {value: "Email"}});
    fireEvent.change(screen.getByLabelText("Contact address"), {target: {value: "mailto:anna@example.com"}});
    fireEvent.click(screen.getByRole("button", {name: "Save draft"}));
    await screen.findByText("Draft saved. Publish to make these changes public.");
    expect(api.PUT).toHaveBeenCalledWith("/api/sites/{id}/draft", expect.objectContaining({headers: {"X-CSRF-Token": "csrf"}, body: expect.objectContaining({displayName: "Anna", template: "editorial", hidePrices: true, photos: [{id: "photo", alt: "A portrait in sunlight", category: ""}], contacts: [{label: "Email", url: "mailto:anna@example.com"}]})}));
    expect(screen.getByRole("link", {name: /Preview saved/}).getAttribute("href")).toBe("/preview/site");
});
test("failed saves preserve edits", async () => {
    vi.mocked(api.GET).mockResolvedValueOnce({response: new Response(), data: draft});
    vi.mocked(api.GET).mockResolvedValueOnce({response: new Response(), data: []});
    vi.mocked(api.PUT).mockRejectedValue(new Error("Draft changed in another window"));
    render(<Editor siteId="site" csrf="csrf"/>);
    fireEvent.change(await screen.findByLabelText("Display name"), {target: {value: "Unsaved Anna"}});
    fireEvent.click(screen.getByRole("button", {name: "Save draft"}));
    await screen.findByRole("alert");
    expect((screen.getByLabelText("Display name") as HTMLInputElement).value).toBe("Unsaved Anna");
    expect(screen.getByText("Unsaved changes")).toBeTruthy();
});

test("reload disables editing until the saved draft arrives", async () => {
    vi.mocked(api.GET).mockResolvedValueOnce({response: new Response(), data: draft});
    vi.mocked(api.GET).mockResolvedValueOnce({response: new Response(), data: []});
    render(<Editor siteId="site" csrf="csrf"/>);
    await screen.findByLabelText("Display name");
    let finish!: (value: {response: Response; data: typeof draft}) => void;
    vi.mocked(api.GET).mockReturnValueOnce(new Promise(resolve => {finish = resolve;}));
    vi.mocked(api.GET).mockResolvedValueOnce({response: new Response(), data: []});
    fireEvent.click(screen.getByRole("button", {name: "Reload saved draft"}));
    expect(screen.getByLabelText("Display name").closest("fieldset")?.disabled).toBe(true);
    expect((screen.getByRole("button", {name: "Please wait…"}) as HTMLButtonElement).disabled).toBe(true);
    finish({response: new Response(), data: {...draft, displayName: "Reloaded Anna"}});
    await screen.findByDisplayValue("Reloaded Anna");
    expect(screen.getByLabelText("Display name").closest("fieldset")?.disabled).toBe(false);
});


test("language changes preserve edits and are saved with the portfolio", async () => {
    vi.mocked(api.GET).mockResolvedValueOnce({response: new Response(), data: draft});
    vi.mocked(api.GET).mockResolvedValueOnce({response: new Response(), data: []});
    vi.mocked(api.PUT).mockResolvedValue({response: new Response(), data: {...draft, language: "ru", version: 1, displayName: "Anna"}});
    render(<Editor siteId="site" csrf="csrf"/>);
    fireEvent.change(await screen.findByLabelText("Display name"), {target: {value: "Anna"}});
    act(() => setLanguage("ru"));
    expect((screen.getByLabelText("Имя") as HTMLInputElement).value).toBe("Anna");
    expect(screen.getByText("Есть несохранённые изменения")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", {name: "Сохранить черновик"}));
    await screen.findByText("Черновик сохранён. Опубликуйте его, чтобы посетители увидели изменения.");
    expect(api.PUT).toHaveBeenCalledWith("/api/sites/{id}/draft", expect.objectContaining({body: expect.objectContaining({language: "ru", displayName: "Anna"})}));
    expect(screen.getByText("Черновик сохранён")).toBeTruthy();
    act(() => setLanguage("en"));
    expect(screen.getByText("Unsaved changes")).toBeTruthy();
    expect(screen.getByText("Draft saved. Publish to make these changes public.")).toBeTruthy();
});

test("a saved portfolio restores the shared language", async () => {
    vi.mocked(api.GET).mockResolvedValueOnce({response: new Response(), data: {...draft, language: "ru", version: 1}});
    vi.mocked(api.GET).mockResolvedValueOnce({response: new Response(), data: []});
    render(<Editor siteId="site" csrf="csrf"/>);
    await screen.findByLabelText("Имя");
    expect(document.documentElement.lang).toBe("ru");
    expect(screen.getByText("Черновик сохранён")).toBeTruthy();
});
