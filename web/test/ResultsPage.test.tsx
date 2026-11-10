import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ResultsPage } from "../src/features/results/ResultsPage";
import { LoadError } from "../src/features/results/data/load";
import type { Data, Summary } from "../src/features/results/data/types";

const row = (source: string, recall: number, budget = 4000): Summary => ({
  source, budget, tasks: 10, file_recall: recall, tasks_with_every_file: 2, owner_recall: 0.85, convention_recall: 0.9, stale_as_fact: source === "fleet-code-context" ? 0 : 15, mean_tokens: 2481,
});
const fleet = (name: "crowded" | "fixture") => ({
  name, repos: name === "crowded" ? 309 : 9, files: 100,
  summary: ["agents-md", "grep-repo", "grep-fleet", "fleet-code-context"].flatMap((s) => [2000, 4000, 8000].map((b) => row(s, s === "fleet-code-context" ? 0.57 : 0.36, b))),
  drift: { claims: 941, checked: 938, flagged: 7, planted: 7, caught: 7, false_flags: [] },
});
const data: Data = { results: { generated: "2026-11-10", neighbours: 300, dev: [fleet("fixture"), fleet("crowded")], heldout: [fleet("fixture"), fleet("crowded")] },
  bench: [{ repos: 309, files: 3209, full_index_seconds: 0.21, one_repo_changed_seconds: 0.09, heap_mb: 12, context_pack_p50_ms: 8, context_pack_p95_ms: 15.4 }] };

describe("results dashboard", () => {
  it("shows a skeleton, then the comparison", async () => {
    render(<ResultsPage load={async () => data} />);
    expect(screen.getByLabelText("Loading the results")).toBeTruthy();
    await screen.findByText("57%");
    expect(screen.getByText("7 of 7")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Held-out, run once" }).getAttribute("aria-pressed")).toBe("true");
  });

  it("explains an error and retries", async () => {
    const load = vi.fn().mockRejectedValueOnce(new LoadError("results.json answered 404.")).mockResolvedValue(data);
    render(<ResultsPage load={load} />);
    await screen.findByText("results.json answered 404.");
    fireEvent.click(screen.getByText("Try again"));
    await screen.findByText("57%");
  });

  it("has an empty state before any evaluation", async () => {
    render(<ResultsPage load={async () => ({ ...data, results: { ...data.results, dev: [], heldout: [] } })} />);
    await screen.findByText("No evaluation has run yet");
  });
});
