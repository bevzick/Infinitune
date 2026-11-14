import { type FormEvent, useState } from "react";

import type {
  DynamicPlaylistInput,
  PlaylistVisibility,
  RuleConditionInput,
  RuleField,
  RuleLogic,
  RuleOperator,
  RuleSet,
  RuleSetInput,
  RuleValue,
} from "./playlistApi";

type DraftCondition = {
  key: string;
  field: RuleField;
  operator: RuleOperator;
  value: string;
  maximum: string;
};

type RuleDraft = {
  name: string;
  logic: RuleLogic;
  conditions: DraftCondition[];
};

const fields: Array<{ value: RuleField; label: string }> = [
  { value: "title", label: "Track title" },
  { value: "artist_id", label: "Artist ID" },
  { value: "album_id", label: "Album ID" },
  { value: "genre_id", label: "Genre ID" },
  { value: "mood_id", label: "Mood ID" },
  { value: "release_year", label: "Release year" },
  { value: "duration_ms", label: "Duration (milliseconds)" },
  { value: "explicit", label: "Explicit content" },
];

const numericOperators: Array<{ value: RuleOperator; label: string }> = [
  { value: "equals", label: "equals" },
  { value: "greater_than", label: "is greater than" },
  { value: "greater_than_or_equal", label: "is at least" },
  { value: "less_than", label: "is less than" },
  { value: "less_than_or_equal", label: "is at most" },
  { value: "between", label: "is between" },
];

export function DynamicPlaylistForm({
  busy,
  onCancel,
  onSubmit,
}: {
  busy?: boolean;
  onCancel(): void;
  onSubmit(input: DynamicPlaylistInput): Promise<void> | void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<PlaylistVisibility>("private");
  const [rules, setRules] = useState<RuleDraft>(() => newRuleDraft());

  return (
    <form
      className="dynamic-playlist-form"
      onSubmit={(event) => {
        event.preventDefault();
        void onSubmit({
          name,
          description,
          visibility,
          criteria: ruleInputFromDraft(rules),
        });
      }}
    >
      <div className="playlist-form dynamic-playlist-details">
        <label>
          Name
          <input
            value={name}
            onChange={(event) => setName(event.target.value)}
            maxLength={150}
            required
          />
        </label>
        <label>
          Visibility
          <select
            value={visibility}
            onChange={(event) =>
              setVisibility(event.target.value as PlaylistVisibility)
            }
          >
            <option value="private">Private</option>
            <option value="unlisted">Unlisted</option>
            <option value="public">Public</option>
          </select>
        </label>
        <label className="playlist-description-field">
          Description
          <textarea
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            maxLength={500}
            rows={3}
          />
        </label>
      </div>
      <RuleFields draft={rules} onChange={setRules} />
      <div className="playlist-form-actions">
        <button type="submit" disabled={busy}>
          {busy ? "Creating…" : "Create dynamic playlist"}
        </button>
        <button
          type="button"
          className="secondary-button"
          onClick={onCancel}
          disabled={busy}
        >
          Cancel
        </button>
      </div>
    </form>
  );
}

export function RuleEditor({
  initial,
  busy,
  onCancel,
  onSubmit,
}: {
  initial: RuleSet;
  busy?: boolean;
  onCancel(): void;
  onSubmit(input: RuleSetInput): Promise<void> | void;
}) {
  const [draft, setDraft] = useState<RuleDraft>(() =>
    draftFromRuleSet(initial),
  );

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void onSubmit(ruleInputFromDraft(draft));
  }

  return (
    <form className="dynamic-playlist-form" onSubmit={submit}>
      <RuleFields draft={draft} onChange={setDraft} />
      <div className="playlist-form-actions">
        <button type="submit" disabled={busy}>
          {busy ? "Saving…" : "Save rules"}
        </button>
        <button
          type="button"
          className="secondary-button"
          onClick={onCancel}
          disabled={busy}
        >
          Cancel
        </button>
      </div>
    </form>
  );
}

function RuleFields({
  draft,
  onChange,
}: {
  draft: RuleDraft;
  onChange(value: RuleDraft): void;
}) {
  function updateCondition(index: number, patch: Partial<DraftCondition>) {
    onChange({
      ...draft,
      conditions: draft.conditions.map((condition, conditionIndex) =>
        conditionIndex === index ? { ...condition, ...patch } : condition,
      ),
    });
  }

  return (
    <fieldset className="rule-builder">
      <legend>Automatic selection rules</legend>
      <p>
        The playlist refreshes from the catalog whenever these rules are read.
      </p>
      <div className="rule-heading-fields">
        <label>
          Rule set name
          <input
            value={draft.name}
            onChange={(event) =>
              onChange({ ...draft, name: event.target.value })
            }
            maxLength={150}
            required
          />
        </label>
        <label>
          Match
          <select
            value={draft.logic}
            onChange={(event) =>
              onChange({
                ...draft,
                logic: event.target.value as RuleLogic,
              })
            }
          >
            <option value="AND">Every condition</option>
            <option value="OR">Any condition</option>
          </select>
        </label>
      </div>

      <div className="rule-condition-list">
        {draft.conditions.map((condition, index) => (
          <div className="rule-condition" key={condition.key}>
            <span className="rule-condition-number" aria-hidden="true">
              {index + 1}
            </span>
            <label>
              Field
              <select
                aria-label={`Condition ${index + 1} field`}
                value={condition.field}
                onChange={(event) => {
                  const field = event.target.value as RuleField;
                  updateCondition(index, defaultCondition(field));
                }}
              >
                {fields.map((field) => (
                  <option value={field.value} key={field.value}>
                    {field.label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Operator
              <select
                aria-label={`Condition ${index + 1} operator`}
                value={condition.operator}
                onChange={(event) =>
                  updateCondition(index, {
                    operator: event.target.value as RuleOperator,
                  })
                }
              >
                {operatorsFor(condition.field).map((operator) => (
                  <option value={operator.value} key={operator.value}>
                    {operator.label}
                  </option>
                ))}
              </select>
            </label>
            <RuleValueField
              condition={condition}
              index={index}
              onChange={(patch) => updateCondition(index, patch)}
            />
            <button
              type="button"
              className="rule-remove"
              aria-label={`Remove condition ${index + 1}`}
              disabled={draft.conditions.length === 1}
              onClick={() =>
                onChange({
                  ...draft,
                  conditions: draft.conditions.filter(
                    (_, conditionIndex) => conditionIndex !== index,
                  ),
                })
              }
            >
              Remove
            </button>
          </div>
        ))}
      </div>
      <button
        type="button"
        className="secondary-button rule-add"
        onClick={() =>
          onChange({
            ...draft,
            conditions: [...draft.conditions, newCondition("title")],
          })
        }
      >
        + Add condition
      </button>
    </fieldset>
  );
}

function RuleValueField({
  condition,
  index,
  onChange,
}: {
  condition: DraftCondition;
  index: number;
  onChange(patch: Partial<DraftCondition>): void;
}) {
  const label = condition.operator === "between" ? "Minimum" : "Value";
  if (condition.field === "explicit") {
    return (
      <label>
        Value
        <select
          aria-label={`Condition ${index + 1} value`}
          value={condition.value}
          onChange={(event) => onChange({ value: event.target.value })}
        >
          <option value="false">Clean only</option>
          <option value="true">Explicit only</option>
        </select>
      </label>
    );
  }

  const numeric =
    condition.field === "release_year" || condition.field === "duration_ms";
  return (
    <div className="rule-value-fields">
      <label>
        {label}
        <input
          aria-label={`Condition ${index + 1} ${label.toLowerCase()}`}
          type={numeric ? "number" : "text"}
          value={condition.value}
          onChange={(event) => onChange({ value: event.target.value })}
          min={numeric ? 1 : undefined}
          max={condition.field === "release_year" ? 9999 : undefined}
          placeholder={valuePlaceholder(condition)}
          required
        />
      </label>
      {condition.operator === "between" ? (
        <label>
          Maximum
          <input
            aria-label={`Condition ${index + 1} maximum`}
            type="number"
            value={condition.maximum}
            onChange={(event) => onChange({ maximum: event.target.value })}
            min={1}
            max={condition.field === "release_year" ? 9999 : undefined}
            required
          />
        </label>
      ) : null}
    </div>
  );
}

export function ruleInputFromDraft(draft: RuleDraft): RuleSetInput {
  return {
    name: draft.name.trim(),
    logic: draft.logic,
    conditions: draft.conditions.map(conditionInputFromDraft),
  };
}

function conditionInputFromDraft(
  condition: DraftCondition,
): RuleConditionInput {
  return {
    field: condition.field,
    operator: condition.operator,
    value: ruleValueFromDraft(condition),
  };
}

function ruleValueFromDraft(condition: DraftCondition): RuleValue {
  if (condition.field === "title") {
    return { kind: "text", text: condition.value.trim() };
  }
  if (condition.field === "explicit") {
    return { kind: "boolean", boolean: condition.value === "true" };
  }
  if (condition.field === "release_year" || condition.field === "duration_ms") {
    if (condition.operator === "between") {
      return {
        kind: "integer_range",
        integer_range: {
          minimum: Number(condition.value),
          maximum: Number(condition.maximum),
        },
      };
    }
    return { kind: "integer", integer: Number(condition.value) };
  }
  if (condition.operator === "in") {
    return {
      kind: "uuid_list",
      uuid_list: condition.value
        .split(",")
        .map((value) => value.trim())
        .filter(Boolean),
    };
  }
  return { kind: "uuid", uuid: condition.value.trim() };
}

function newRuleDraft(): RuleDraft {
  return {
    name: "My dynamic rules",
    logic: "AND",
    conditions: [newCondition("title")],
  };
}

function draftFromRuleSet(ruleSet: RuleSet): RuleDraft {
  return {
    name: ruleSet.name,
    logic: ruleSet.logic,
    conditions: [...ruleSet.conditions]
      .sort((left, right) => left.position - right.position)
      .map((condition) =>
        draftConditionFromValue(
          condition.field,
          condition.operator,
          condition.value,
        ),
      ),
  };
}

function draftConditionFromValue(
  field: RuleField,
  operator: RuleOperator,
  value: RuleValue,
): DraftCondition {
  let draftValue = "";
  let maximum = "";
  switch (value.kind) {
    case "text":
      draftValue = value.text;
      break;
    case "uuid":
      draftValue = value.uuid;
      break;
    case "uuid_list":
      draftValue = value.uuid_list.join(", ");
      break;
    case "integer":
      draftValue = String(value.integer);
      break;
    case "integer_range":
      draftValue = String(value.integer_range.minimum);
      maximum = String(value.integer_range.maximum);
      break;
    case "boolean":
      draftValue = String(value.boolean);
      break;
  }
  return {
    key: crypto.randomUUID(),
    field,
    operator,
    value: draftValue,
    maximum,
  };
}

function newCondition(field: RuleField): DraftCondition {
  return {
    key: crypto.randomUUID(),
    ...defaultCondition(field),
  };
}

function defaultCondition(field: RuleField) {
  return {
    field,
    operator: defaultOperator(field),
    value: field === "explicit" ? "false" : "",
    maximum: "",
  };
}

function defaultOperator(field: RuleField): RuleOperator {
  if (field === "title") return "contains";
  return "equals";
}

function operatorsFor(field: RuleField) {
  if (field === "title") {
    return [
      { value: "contains" as const, label: "contains" },
      { value: "equals" as const, label: "equals" },
    ];
  }
  if (field === "release_year" || field === "duration_ms") {
    return numericOperators;
  }
  if (field === "explicit") {
    return [{ value: "equals" as const, label: "equals" }];
  }
  return [
    { value: "equals" as const, label: "equals" },
    { value: "in" as const, label: "is one of" },
  ];
}

function valuePlaceholder(condition: DraftCondition) {
  if (condition.field === "title") return "e.g. live";
  if (condition.field === "release_year") return "e.g. 2020";
  if (condition.field === "duration_ms") return "e.g. 180000";
  return condition.operator === "in" ? "Comma-separated UUIDs" : "Catalog UUID";
}
