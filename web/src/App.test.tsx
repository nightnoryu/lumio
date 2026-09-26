// @vitest-environment jsdom
import {fireEvent, render, screen, waitFor, cleanup} from "@testing-library/react";
import {afterEach, beforeEach, expect, test, vi} from "vitest";
import {App} from "./App";
import {api} from "./api/client";

vi.mock("./api/client", () => ({
    api: {GET: vi.fn(), POST: vi.fn()},
    failure: (error: unknown) => error instanceof Error ? error : new Error("Request failed"),
}));

const viewer = {id: "owner", email: "anna@example.com", csrfToken: "csrf-secret", baseDomain: "example.com"};
beforeEach(() => vi.resetAllMocks());
afterEach(cleanup);

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
    expect(screen.getByText("Private draft")).toBeTruthy();
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
