export const SOURCES = ["agents-md", "grep-repo", "grep-fleet", "fleet-code-context"] as const;

export const LABEL: Record<string, string> = {
  "agents-md": "AGENTS.md of the repo it starts in",
  "grep-repo": "AGENTS.md + grep its own repo",
  "grep-fleet": "AGENTS.md + grep the whole fleet",
  "fleet-code-context": "fleet-code-context",
};

export const OURS = "fleet-code-context";
