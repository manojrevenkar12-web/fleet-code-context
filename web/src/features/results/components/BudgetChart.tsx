import { color, font, layout, size, space } from "../../../shared/theme/tokens";
import { percent } from "../../../shared/format/format";
import { OURS } from "../data/labels";
import type { Summary } from "../data/types";

const BUDGETS = [2000, 4000, 8000];
const PAD = { left: 44, right: 160, top: 12, bottom: 28 };

/** Files found as the token budget grows, for fleet-wide grep and for the context pack. */
export function BudgetChart({ summary }: { summary: Summary[] }) {
  const w = layout.chartWidth, h = layout.chartHeight;
  const x = (i: number) => PAD.left + (i * (w - PAD.left - PAD.right)) / (BUDGETS.length - 1);
  const y = (v: number) => PAD.top + (1 - v) * (h - PAD.top - PAD.bottom);
  const series = [
    { src: "grep-fleet", label: "grep the whole fleet", stroke: color.baseline },
    { src: OURS, label: "fleet-code-context", stroke: color.ours },
  ];
  return (
    <svg viewBox={`0 0 ${w} ${h}`} role="img" aria-label="Share of required files found at 2,000, 4,000 and 8,000 tokens" style={{ width: "100%", maxWidth: w, fontFamily: font.family }}>
      {[0, 0.5, 1].map((v) => (
        <g key={v}>
          <line x1={PAD.left} x2={w - PAD.right} y1={y(v)} y2={y(v)} stroke={color.rule} />
          <text x={PAD.left - space.s} y={y(v) + 4} textAnchor="end" fontSize={size.small} fill={color.muted}>{percent(v)}</text>
        </g>
      ))}
      {BUDGETS.map((b, i) => (
        <text key={b} x={x(i)} y={h - space.s} textAnchor="middle" fontSize={size.small} fill={color.muted}>{b / 1000}k tokens</text>
      ))}
      {series.map((s) => {
        const pts = BUDGETS.map((b) => summary.find((r) => r.source === s.src && r.budget === b)?.file_recall ?? 0);
        const last = pts[pts.length - 1];
        return (
          <g key={s.src}>
            <polyline points={pts.map((v, i) => `${x(i)},${y(v)}`).join(" ")} fill="none" stroke={s.stroke} strokeWidth={2.5} />
            {pts.map((v, i) => <circle key={i} cx={x(i)} cy={y(v)} r={3.5} fill={s.stroke} />)}
            <text x={x(BUDGETS.length - 1) + space.s} y={y(last) + 4} fontSize={size.small} fill={s.stroke}>{s.label}</text>
          </g>
        );
      })}
    </svg>
  );
}
