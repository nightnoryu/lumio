import createClient from "openapi-fetch";
import type {paths, components} from "./schema";

export type Viewer = components["schemas"]["Viewer"];
export type Site = components["schemas"]["Site"];
export const api = createClient<paths>({credentials: "same-origin"});
export function failure(error: unknown): Error {
    if (error && typeof error === "object" && "message" in error && typeof error.message === "string") {
        return new Error(error.message);
    }
    return new Error("The request failed. Please try again.");
}
