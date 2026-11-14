import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { RuleEditor } from "./DynamicRuleForm";
import type { RuleSetInput } from "./playlistApi";

afterEach(cleanup);

describe("RuleEditor", () => {
  it("preserves typed boolean and numeric range values", async () => {
    const onSubmit = vi.fn<(input: RuleSetInput) => void>();
    const user = userEvent.setup();
    render(
      <RuleEditor
        initial={{
          id: "rule-set-id",
          owner_id: "owner-id",
          name: "Recent clean tracks",
          logic: "AND",
          conditions: [
            {
              id: "condition-one",
              position: 1,
              field: "release_year",
              operator: "between",
              value: {
                kind: "integer_range",
                integer_range: { minimum: 2020, maximum: 2026 },
              },
            },
            {
              id: "condition-two",
              position: 2,
              field: "explicit",
              operator: "equals",
              value: { kind: "boolean", boolean: false },
            },
          ],
          created_at: "2026-11-10T00:00:00Z",
          updated_at: "2026-11-10T00:00:00Z",
        }}
        onCancel={() => undefined}
        onSubmit={onSubmit}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Save rules" }));

    expect(onSubmit).toHaveBeenCalledWith({
      name: "Recent clean tracks",
      logic: "AND",
      conditions: [
        {
          field: "release_year",
          operator: "between",
          value: {
            kind: "integer_range",
            integer_range: { minimum: 2020, maximum: 2026 },
          },
        },
        {
          field: "explicit",
          operator: "equals",
          value: { kind: "boolean", boolean: false },
        },
      ],
    });
  });
});
