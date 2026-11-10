import { useState } from "react";
import { count } from "../../shared/format/format";
import { color, layout, size, space, weight } from "../../shared/theme/tokens";
import { Button } from "../../shared/ui/Button";
import { Notice } from "../../shared/ui/Notice";
import { Section } from "../../shared/ui/Section";
import { BudgetChart } from "./components/BudgetChart";
import { FactsTable } from "./components/FactsTable";
import { RecallBars } from "./components/RecallBars";
import { ResultsSkeleton } from "./components/ResultsSkeleton";
import { ScaleTable } from "./components/ScaleTable";
import { Toggle } from "./components/Toggle";
import { loadData } from "./data/load";
import type { Data } from "./data/types";
import { useData } from "./data/useData";

type Split = "heldout" | "dev";
type Fleet = "crowded" | "fixture";
const BUDGET = 4000;

export function ResultsPage({ load = loadData }: { load?: typeof loadData }) {
  const { state, retry } = useData(load);
  return (
    <main style={{ maxWidth: layout.maxWidth, margin: "0 auto", padding: `${space.xl}px ${space.l}px ${space.xxl}px` }}>
      {state.status === "loading" && <ResultsSkeleton />}
      {state.status === "error" && (
        <Notice tone="error" title="The results did not load" action={<Button onClick={retry}>Try again</Button>}>
          {state.message}
        </Notice>
      )}
      {state.status === "ready" && state.data.results.dev.length === 0 && (
        <Notice tone="empty" title="No evaluation has run yet">
          Run <code>make eval report</code> to score the context sources, then reload.
        </Notice>
      )}
      {state.status === "ready" && state.data.results.dev.length > 0 && <Results data={state.data} />}
    </main>
  );
}

function Results({ data }: { data: Data }) {
  const hasHeld = (data.results.heldout?.length ?? 0) > 0;
  const [split, setSplit] = useState<Split>(hasHeld ? "heldout" : "dev");
  const [fleet, setFleet] = useState<Fleet>("crowded");
  const fr = (split === "heldout" ? data.results.heldout : data.results.dev) ?? [];
  const f = fr.find((x) => x.name === fleet);
  const at = f?.summary.filter((s) => s.budget === BUDGET) ?? [];
  const tickets = at[0]?.tasks ?? 0;
  const crowded = data.results.dev.find((x) => x.name === "crowded");
  return (
    <>
      <header>
        <h1 style={{ fontSize: size.heading * 1.4, lineHeight: 1.15, fontWeight: weight.bold, margin: 0, maxWidth: "24ch", color: color.ink }}>
          Of the files a correct change needs, how many reach the coding agent?
        </h1>
        <p style={{ color: color.muted, maxWidth: "62ch", margin: `${space.m}px 0 ${space.l}px` }}>
          {tickets} tickets whose correct change is known, {BUDGET.toLocaleString("en-GB")} tokens of context each.
          The crowded fleet is {count(crowded?.repos ?? 0)} repositories: the 9-repository fleet inside {data.results.neighbours} neighbour
          services that share its words, events and RPCs.
        </p>
        <div style={{ display: "flex", gap: space.m, flexWrap: "wrap", marginBottom: space.l }}>
          <Toggle label="Tickets" value={split} onChange={setSplit}
            options={[...(hasHeld ? [{ value: "heldout" as Split, text: "Held-out, run once" }] : []), { value: "dev", text: "Dev, tuned on" }]} />
          <Toggle label="Fleet" value={fleet} onChange={setFleet}
            options={[{ value: "crowded", text: `Crowded, ${count(crowded?.repos ?? 0)} repos` }, { value: "fixture", text: "Fixture, 9 repos" }]} />
        </div>
      </header>
      {f ? <RecallBars rows={at} /> : <p style={{ color: color.muted }}>No results for this choice.</p>}
      {f && (
        <>
          <Section title="Beyond the files" note="What the agent is told about conventions, reviewers and stale instructions.">
            <FactsTable rows={at} />
          </Section>
          <Section title="As the budget grows" note="Share of required files found at each token budget.">
            <BudgetChart summary={f.summary} />
          </Section>
          <Section title="Instructions that no longer hold">
            <p style={{ margin: 0, maxWidth: "62ch" }}>
              {count(f.drift.claims)} AGENTS.md instructions read, {count(f.drift.checked)} checked against the code.{" "}
              <strong style={{ color: color.ours }}>{f.drift.caught} of {f.drift.planted}</strong> planted stale instructions caught,{" "}
              <strong>{f.drift.false_flags?.length ?? 0}</strong> false alarms.
            </p>
          </Section>
        </>
      )}
      <Section title="At fleet scale" note="Indexing and answering on one machine, as the fleet grows.">
        <ScaleTable rows={data.bench} />
      </Section>
    </>
  );
}
