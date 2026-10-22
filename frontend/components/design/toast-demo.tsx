"use client";

import { Button } from "@/components/ui/button";
import { ToastProvider, useToast } from "@/components/ui/toast";

function Buttons() {
  const { show } = useToast();
  return (
    <div className="flex flex-wrap gap-3">
      <Button
        variant="outline"
        onClick={() =>
          show({ tone: "success", title: "Changes saved", description: "Your draft was saved." })
        }
      >
        Show success toast
      </Button>
      <Button
        variant="outline"
        onClick={() =>
          show({
            tone: "danger",
            title: "Could not save",
            description: "Check your connection and try again.",
          })
        }
      >
        Show error toast
      </Button>
    </div>
  );
}

export function ToastDemo() {
  return (
    <ToastProvider>
      <Buttons />
    </ToastProvider>
  );
}
