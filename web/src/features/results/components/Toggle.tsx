import { color, font, layout, size, space, weight } from "../../../shared/theme/tokens";

/** A segmented choice; the selected option is a pressed button. */
export function Toggle<T extends string>({ label, value, options, onChange }: { label: string; value: T; options: { value: T; text: string }[]; onChange: (v: T) => void }) {
  return (
    <div role="group" aria-label={label} style={{ display: "inline-flex", border: `1px solid ${color.rule}`, borderRadius: layout.radius, overflow: "hidden" }}>
      {options.map((o) => {
        const on = o.value === value;
        return (
          <button key={o.value} type="button" aria-pressed={on} onClick={() => onChange(o.value)}
            style={{ fontFamily: font.family, fontSize: size.small, fontWeight: on ? weight.medium : weight.regular, padding: `${space.s}px ${space.m}px`,
              border: 0, background: on ? color.ink : "transparent", color: on ? color.surface : color.ink, cursor: "pointer" }}>
            {o.text}
          </button>
        );
      })}
    </div>
  );
}
