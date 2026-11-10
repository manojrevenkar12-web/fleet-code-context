import { useCallback, useEffect, useState } from "react";
import { LoadError, loadData } from "./load";
import type { State } from "./types";

export function useData(load = loadData) {
  const [state, setState] = useState<State>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const ctl = new AbortController();
    setState({ status: "loading" });
    load(ctl.signal)
      .then((data) => setState({ status: "ready", data }))
      .catch((e: unknown) => {
        if (!ctl.signal.aborted) setState({ status: "error", message: e instanceof LoadError ? e.message : "The results could not be read." });
      });
    return () => ctl.abort();
  }, [attempt, load]);
  return { state, retry: useCallback(() => setAttempt((n) => n + 1), []) };
}
