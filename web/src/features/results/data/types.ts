export interface Summary {
  source: string;
  budget: number;
  tasks: number;
  file_recall: number;
  tasks_with_every_file: number;
  owner_recall: number;
  convention_recall: number;
  stale_as_fact: number;
  mean_tokens: number;
}

export interface FleetResult {
  name: "fixture" | "crowded";
  repos: number;
  files: number;
  summary: Summary[];
  drift: { claims: number; checked: number; flagged: number; planted: number; caught: number; false_flags?: string[] };
}

export interface Results {
  generated: string;
  neighbours: number;
  dev: FleetResult[];
  heldout?: FleetResult[];
}

export interface BenchRow {
  repos: number;
  files: number;
  full_index_seconds: number;
  one_repo_changed_seconds: number;
  heap_mb: number;
  context_pack_p50_ms: number;
  context_pack_p95_ms: number;
}

export interface Data {
  results: Results;
  bench: BenchRow[];
}

export type State = { status: "loading" } | { status: "error"; message: string } | { status: "ready"; data: Data };
