// @vitest-environment jsdom
import {cleanup, fireEvent, render, screen} from "@testing-library/react";
import {afterEach, expect, test, vi} from "vitest";
import {Publishing} from "./Publishing";
import {api} from "./api/client";
vi.mock("./api/client", () => ({api: {GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn()}, failure: (e: unknown) => e instanceof Error ? e : new Error("Publish failed")}));
afterEach(() => {cleanup(); vi.resetAllMocks();});
const state = {published: false, version: 0, url: "https://anna.lumio.test/"};
test("publish a saved version and unpublish the live site", async () => {
    vi.mocked(api.GET).mockResolvedValue({response: new Response(), data: state});
    vi.mocked(api.POST).mockResolvedValue({response: new Response(), data: {...state, published: true, version: 3}});
    vi.mocked(api.DELETE).mockResolvedValue({response: new Response(null, {status: 204}), data: undefined});
    render(<Publishing siteId="site" csrf="secret" version={3} dirty={false} saving={false}/>);
    fireEvent.click(await screen.findByRole("button", {name: "Publish portfolio"}));
    await screen.findByRole("link", {name: /Open live portfolio/});
    expect(api.POST).toHaveBeenCalledWith("/api/sites/{id}/publication", expect.objectContaining({body: {version: 3}, headers: {"X-CSRF-Token": "secret", "Content-Type": "application/json"}}));
    fireEvent.click(screen.getByRole("button", {name: "Unpublish portfolio"}));
    await screen.findByText("Portfolio unpublished. Visitors can no longer open it.");
    expect(screen.queryByRole("link", {name: /Open live portfolio/})).toBeNull();
});
test("unsaved edits cannot be published and a failed publish keeps the live state", async () => {
    vi.mocked(api.GET).mockResolvedValue({response: new Response(), data: state});
    vi.mocked(api.POST).mockRejectedValue(new Error("The saved draft changed"));
    const view = render(<Publishing siteId="site" csrf="secret" version={3} dirty saving={false}/>);
    expect((await screen.findByRole("button", {name: "Publish portfolio"}) as HTMLButtonElement).disabled).toBe(true);
    view.rerender(<Publishing siteId="site" csrf="secret" version={3} dirty={false} saving={false}/>);
    fireEvent.click(screen.getByRole("button", {name: "Publish portfolio"}));
    await screen.findByText("The saved draft changed");
    expect(screen.getByText("Private portfolio")).toBeTruthy();
});
