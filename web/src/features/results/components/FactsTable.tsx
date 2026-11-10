import { percent } from "../../../shared/format/format";
import { color, font, size, space, weight } from "../../../shared/theme/tokens";
import { LABEL, OURS, SOURCES } from "../data/labels";
import type { Summary } from "../data/types";

const cell = { padding: `${space.s}px ${space.s}px`, textAlign: "right" as const };

export function FactsTable({ rows }: { rows: Summary[] }) {
  return (
    <div style={{ overflowX: "auto" }}>
      <table style={{ width: "100%", borderCollapse: "collapse", fontVariantNumeric: font.numerals, minWidth: 560 }}>
        <thead>
          <tr style={{ color: color.muted, fontSize: size.small }}>
            <th style={{ ...cell, textAlign: "left", paddingLeft: 0, fontWeight: weight.regular }}>Context the agent gets</th>
            <th style={{ ...cell, fontWeight: weight.regular }}>Every file found</th>
            <th style={{ ...cell, fontWeight: weight.regular }}>Conventions</th>
            <th style={{ ...cell, fontWeight: weight.regular }}>Reviewing teams</th>
            <th style={{ ...cell, fontWeight: weight.regular }}>Stale instructions passed on</th>
            <th style={{ ...cell, paddingRight: 0, fontWeight: weight.regular }}>Tokens</th>
          </tr>
        </thead>
        <tbody>
          {SOURCES.map((src) => {
            const r = rows.find((x) => x.source === src);
            if (!r) return null;
            const ours = src === OURS;
            return (
              <tr key={src} style={{ borderTop: `1px solid ${color.rule}`, fontWeight: ours ? weight.medium : weight.regular }}>
                <td style={{ ...cell, textAlign: "left", paddingLeft: 0 }}>{LABEL[src]}</td>
                <td style={cell}>{r.tasks_with_every_file} of {r.tasks}</td>
                <td style={cell}>{percent(r.convention_recall)}</td>
                <td style={cell}>{percent(r.owner_recall)}</td>
                <td style={{ ...cell, color: r.stale_as_fact > 0 ? color.stale : color.ours }}>{r.stale_as_fact}</td>
                <td style={{ ...cell, paddingRight: 0 }}>{Math.round(r.mean_tokens).toLocaleString("en-GB")}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
