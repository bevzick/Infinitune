import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { DynamicPlaylistForm, RuleEditor } from "./DynamicRuleForm";
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

describe("DynamicPlaylistForm", () => {
  it("submits genre and artist names as text, not UUIDs", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(
      <DynamicPlaylistForm onCancel={() => undefined} onSubmit={onSubmit} />,
    );

    await user.type(
      screen.getByLabelText("Name", { exact: true }),
      "Hip Hop discovery",
    );
    await user.selectOptions(
      screen.getByLabelText("Condition 1 field"),
      "genre",
    );
    await user.type(screen.getByLabelText("Condition 1 value"), "Hip Hop");
    await user.click(screen.getByRole("button", { name: /Add condition/ }));
    await user.selectOptions(
      screen.getByLabelText("Condition 2 field"),
      "artist",
    );
    await user.selectOptions(
      screen.getByLabelText("Condition 2 operator"),
      "contains",
    );
    await user.type(screen.getByLabelText("Condition 2 value"), "Hill");
    await user.click(
      screen.getByRole("button", { name: "Create dynamic playlist" }),
    );

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        criteria: {
          name: "My dynamic rules",
          logic: "AND",
          conditions: [
            {
              field: "genre",
              operator: "equals",
              value: { kind: "text", text: "Hip Hop" },
            },
            {
              field: "artist",
              operator: "contains",
              value: { kind: "text", text: "Hill" },
            },
          ],
        },
      }),
    );
  });

  it("keeps existing UUID-based rules editable", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    const oldUUID = "11111111-1111-4111-8111-111111111111";
    render(
      <RuleEditor
        initial={{
          id: "rules",
          owner_id: "owner",
          name: "Old rules",
          logic: "AND",
          conditions: [
            {
              id: "condition",
              position: 1,
              field: "genre_id",
              operator: "equals",
              value: { kind: "uuid", uuid: oldUUID },
            },
          ],
          created_at: "2026-11-10",
          updated_at: "2026-11-10",
        }}
        onCancel={() => undefined}
        onSubmit={onSubmit}
      />,
    );

    expect(
      screen.getByRole("option", { name: "genre ID (existing rule)" }),
    ).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Save rules" }));
    expect(onSubmit).toHaveBeenCalledWith({
      name: "Old rules",
      logic: "AND",
      conditions: [
        {
          field: "genre_id",
          operator: "equals",
          value: { kind: "uuid", uuid: oldUUID },
        },
      ],
    });
  });
});
