import { color, font, layout, leading, size, space } from "../shared/theme/tokens";

export function applyGlobalStyles() {
  const el = document.createElement("style");
  el.textContent = `
    body { margin: 0; background: ${color.paper}; color: ${color.ink}; font-family: ${font.family}; font-size: ${size.body}px; line-height: ${leading.body}; }
    :focus-visible { outline: 2px solid ${color.ours}; outline-offset: 2px; }
    code { font-family: inherit; font-weight: 600; }
    .bar-row { display: grid; grid-template-columns: minmax(12rem, 1.3fr) 2fr 4rem; gap: ${space.m}px; align-items: center; }
    @media (max-width: ${layout.narrow}px) {
      .bar-row { grid-template-columns: 1fr auto; }
      .bar-row > :nth-child(2) { grid-column: 1 / -1; grid-row: 2; }
    }
    @keyframes pulse { 50% { opacity: .55; } }
    .pulse { animation: pulse 1.4s ease-in-out infinite; }
    @media (prefers-reduced-motion: reduce) { .pulse { animation: none; } }
  `;
  document.head.appendChild(el);
}
