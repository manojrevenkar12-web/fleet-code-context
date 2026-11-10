/** Every colour, size, space and timing the dashboard uses. */
export const color = {
  paper: "#F2F4F1",
  surface: "#FBFCFA",
  ink: "#1B2421",
  muted: "#5A6662",
  rule: "#D3D9D5",
  ours: "#0F6E66",
  oursSoft: "#CFE5E2",
  baseline: "#8A949C",
  baselineSoft: "#E2E6E8",
  stale: "#B5482C",
  skeleton: "#DDE2DE",
} as const;

export const font = {
  family: '"IBM Plex Sans", system-ui, sans-serif',
  numerals: "lining-nums proportional-nums",
} as const;

export const size = { small: 13, body: 16, lead: 20, heading: 25, figure: 49 } as const;
export const weight = { regular: 400, medium: 500, bold: 700 } as const;
export const leading = { tight: 1.1, body: 1.5 } as const;
export const space = { xs: 4, s: 8, m: 16, l: 24, xl: 40, xxl: 64 } as const;
export const layout = { maxWidth: 1040, barHeight: 12, radius: 3, narrow: 720, chartHeight: 180, chartWidth: 520 } as const;
export const motion = { barSeconds: 0.7, stagger: 0.08, ease: [0.22, 1, 0.36, 1] as [number, number, number, number] } as const;
export const skeletonRows = 4;
