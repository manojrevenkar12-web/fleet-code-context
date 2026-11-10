import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-sans/700.css";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ResultsPage } from "../features/results/ResultsPage";
import { applyGlobalStyles } from "./globalStyles";

applyGlobalStyles();
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ResultsPage />
  </StrictMode>,
);
