import { useEffect } from "react";
import { notifications } from "@mantine/notifications";
import { Severity, AlertColor, AlertTitle } from "@traceviz/client-core";
import type { ConfigurationError } from "@traceviz/client-core";
import { useAppCore } from "../../core/index.ts";

export function ErrorToast(): null {
  const appCore = useAppCore();

  useEffect(() => {
    const sub = appCore.configurationErrors.subscribe(
      (err: ConfigurationError) => {
        const severity: Severity | undefined = err.severity;
        const color = AlertColor(severity ?? Severity.WARNING);
        const title = AlertTitle(severity ?? Severity.WARNING);
        notifications.show({
          title,
          message: err.toString(),
          color,
          autoClose: false,
          withCloseButton: true,
        });
      },
    );
    return () => sub.unsubscribe();
  }, [appCore]);

  return null;
}
