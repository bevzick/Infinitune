import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";

import { FeedbackProvider, useFeedback } from "./FeedbackContext";

describe("FeedbackProvider", () => {
  afterEach(cleanup);

  it("announces and dismisses a success message", async () => {
    const user = userEvent.setup();
    render(
      <FeedbackProvider>
        <FeedbackTrigger />
      </FeedbackProvider>,
    );

    await user.click(screen.getByRole("button", { name: "Notify" }));
    expect(screen.getByRole("status")).toHaveTextContent("Saved to library");

    await user.click(
      screen.getByRole("button", { name: "Dismiss notification" }),
    );
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});

function FeedbackTrigger() {
  const feedback = useFeedback();
  return (
    <button
      type="button"
      onClick={() => feedback?.notify("Saved to library", "success")}
    >
      Notify
    </button>
  );
}
