import { count } from "../../../shared/format/format";
import { color, font, size, space, weight } from "../../../shared/theme/tokens";
import type { BenchRow } from "../data/types";

const cell = { padding: `${space.s}px`, textAlign: "right" as const };

export function ScaleTable({ rows }: { rows: BenchRow[] }) {
  return (
    <div style={{ overflowX: "auto" }}>
      <table style={{ width: "100%", borderCollapse: "collapse", fontVariantNumeric: font.numerals, minWidth: 520 }}>
        <thead>
          <tr style={{ color: color.muted, fontSize: size.small }}>
            {["Repositories", "Files", "Full index", "One repo changed", "Heap", "context_pack p50 / p95"].map((h, i) => (
              <th key={h} style={{ ...cell, fontWeight: weight.regular, textAlign: i === 0 ? "left" : "right", paddingLeft: i === 0 ? 0 : space.s }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.repos} style={{ borderTop: `1px solid ${color.rule}` }}>
              <td style={{ ...cell, textAlign: "left", paddingLeft: 0 }}>{count(r.repos)}</td>
              <td style={cell}>{count(r.files)}</td>
              <td style={cell}>{r.full_index_seconds.toFixed(2)} s</td>
              <td style={cell}>{r.one_repo_changed_seconds.toFixed(2)} s</td>
              <td style={cell}>{Math.round(r.heap_mb)} MB</td>
              <td style={cell}>{r.context_pack_p50_ms.toFixed(1)} / {r.context_pack_p95_ms.toFixed(1)} ms</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
