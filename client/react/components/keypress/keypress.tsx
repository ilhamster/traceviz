import { useEffect, useMemo } from "react";
import {
  ConfigurationError,
  Interactions,
  Keypress,
  StringSetValue,
  ValueMap,
} from "@traceviz/client-core";
import { useAppCore } from "../../core/index.ts";

export const KEY_TARGET = "key";
export const KEY_PRESS_ACTION = "press";
export const DEPRESSED_KEY_CODES_KEY = "depressed_key_codes";

const supportedActions: Array<[string, string]> = [[KEY_TARGET, KEY_PRESS_ACTION]];
const supportedReactions: Array<[string, string]> = [];
const supportedWatches: string[] = [];

export type KeypressListenerProps = {
  interactions?: Interactions;
};

// KeypressListener routes window keydown/keyup events into TraceViz
// interactions without attaching any application-specific key semantics.
export function KeypressListener({ interactions }: KeypressListenerProps): null {
  const appCore = useAppCore();
  const depressedKeyCodes = useMemo(
    () => new StringSetValue(new Set<string>()),
    [],
  );
  const localState = useMemo(
    () =>
      new ValueMap(
        new Map([[DEPRESSED_KEY_CODES_KEY, depressedKeyCodes]]),
      ),
    [depressedKeyCodes],
  );

  useEffect(() => {
    if (!interactions) {
      return;
    }
    try {
      interactions.checkForSupportedActions(supportedActions);
      interactions.checkForSupportedReactions(supportedReactions);
      interactions.checkForSupportedWatches(supportedWatches);
    } catch (err: unknown) {
      appCore.err(
        err instanceof Error ? err : new ConfigurationError(String(err)),
      );
    }
    const keypress = new Keypress(depressedKeyCodes);
    const listener = (event: KeyboardEvent): void => {
      if (
        event.target instanceof HTMLInputElement ||
        event.target instanceof HTMLTextAreaElement ||
        event.target instanceof HTMLSelectElement ||
        (event.target instanceof HTMLElement && event.target.isContentEditable)
      ) {
        return;
      }
      keypress.keyEvent(event);
      try {
        interactions.update(KEY_TARGET, KEY_PRESS_ACTION, localState);
      } catch (err: unknown) {
        appCore.err(
          err instanceof Error ? err : new ConfigurationError(String(err)),
        );
      }
    };
    window.addEventListener("keydown", listener);
    window.addEventListener("keyup", listener);
    return () => {
      window.removeEventListener("keydown", listener);
      window.removeEventListener("keyup", listener);
    };
  }, [appCore, depressedKeyCodes, interactions, localState]);

  return null;
}
