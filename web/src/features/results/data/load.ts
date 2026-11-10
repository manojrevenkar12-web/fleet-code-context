import type { BenchRow, Data, Results } from "./types";

export class LoadError extends Error {}

export async function loadData(signal?: AbortSignal): Promise<Data> {
  const get = async (path: string) => {
    const res = await fetch(path, { signal }).catch(() => {
      throw new LoadError(`Could not reach ${path}. Run \`make report\` and serve the dashboard again.`);
    });
    if (!res.ok) throw new LoadError(`${path} answered ${res.status}. Run \`make report\` to generate it.`);
    return res.json();
  };
  const [results, bench] = await Promise.all([get("/results.json"), get("/bench.json")]);
  if (!Array.isArray((results as Results).dev)) throw new LoadError("results.json is not in the expected shape.");
  return { results: results as Results, bench: ((bench as { rows: BenchRow[] }).rows ?? []) };
}
