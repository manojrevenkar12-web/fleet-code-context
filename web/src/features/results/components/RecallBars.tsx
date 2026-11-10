import { motion, useReducedMotion } from "framer-motion";
import { percent } from "../../../shared/format/format";
import { color, font, layout, motion as m, size, space, weight } from "../../../shared/theme/tokens";
import { LABEL, OURS, SOURCES } from "../data/labels";
import type { Summary } from "../data/types";

/** One bar per context source: the share of the files a correct change needs that reached the agent. */
export function RecallBars({ rows }: { rows: Summary[] }) {
  const reduce = useReducedMotion();
  return (
    <ul style={{ listStyle: "none", margin: 0, padding: 0 }}>
      {SOURCES.map((src, i) => {
        const r = rows.find((x) => x.source === src);
        if (!r) return null;
        const ours = src === OURS;
        return (
          <li key={src} className="bar-row" style={{ padding: `${space.s}px 0`, borderTop: `1px solid ${color.rule}` }}>
            <span style={{ fontWeight: ours ? weight.bold : weight.regular, color: color.ink }}>{LABEL[src]}</span>
            <span aria-hidden style={{ height: layout.barHeight, background: ours ? color.oursSoft : color.baselineSoft, borderRadius: layout.radius, overflow: "hidden" }}>
              <motion.span
                key={`${src}-${r.file_recall}`}
                initial={reduce ? false : { scaleX: 0 }}
                animate={{ scaleX: r.file_recall }}
                transition={{ duration: m.barSeconds, ease: m.ease, delay: reduce ? 0 : i * m.stagger }}
                style={{ display: "block", height: "100%", transformOrigin: "left", background: ours ? color.ours : color.baseline }}
              />
            </span>
            <span style={{ textAlign: "right", fontVariantNumeric: font.numerals, fontSize: ours ? size.lead : size.body, fontWeight: ours ? weight.bold : weight.regular, color: ours ? color.ours : color.ink }}>
              {percent(r.file_recall)}
            </span>
          </li>
        );
      })}
    </ul>
  );
}
