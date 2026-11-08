package rules

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNewBuildsTypedFlatRuleSet(t *testing.T) {
	ownerID := uuid.New()
	artistID := uuid.New()
	genreIDs := []uuid.UUID{uuid.New(), uuid.New()}
	input := CreateInput{
		Name:  "  Morning mix  ",
		Logic: LogicAnd,
		Conditions: []ConditionInput{
			{Field: FieldTitle, Operator: OperatorContains, Value: TextValue("  sunrise  ")},
			{Field: FieldArtistID, Operator: OperatorEquals, Value: UUIDValue(artistID)},
			{Field: FieldGenreID, Operator: OperatorIn, Value: UUIDListValue(genreIDs...)},
			{Field: FieldReleaseYear, Operator: OperatorBetween, Value: IntegerRangeValue(2020, 2026)},
			{Field: FieldDurationMS, Operator: OperatorLessThanOrEqual, Value: IntegerValue(240000)},
			{Field: FieldExplicit, Operator: OperatorEquals, Value: BooleanValue(false)},
		},
	}
	input.Conditions[0].Value.UUID = uuid.New()

	result, err := New(ownerID, input)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if result.ID == uuid.Nil || result.OwnerID != ownerID || result.Name != "Morning mix" ||
		result.Logic != LogicAnd || len(result.Conditions) != len(input.Conditions) {
		t.Fatalf("New() = %+v", result)
	}
	for index, condition := range result.Conditions {
		if condition.ID == uuid.Nil || condition.RuleSetID != result.ID || condition.Position != index+1 {
			t.Fatalf("condition[%d] = %+v", index, condition)
		}
	}
	if result.Conditions[0].Value.Text != "sunrise" {
		t.Fatalf("canonical title = %q", result.Conditions[0].Value.Text)
	}
	if result.Conditions[0].Value.UUID != uuid.Nil {
		t.Fatalf("canonical text value retained unrelated UUID %s", result.Conditions[0].Value.UUID)
	}
	input.Conditions[2].Value.UUIDs[0] = uuid.New()
	if result.Conditions[2].Value.UUIDs[0] == input.Conditions[2].Value.UUIDs[0] {
		t.Fatal("New() retained the caller's mutable UUID list")
	}
}

func TestNewRejectsInvalidRuleSetMetadata(t *testing.T) {
	validCondition := ConditionInput{
		Field: FieldExplicit, Operator: OperatorEquals, Value: BooleanValue(true),
	}
	tests := []struct {
		name    string
		ownerID uuid.UUID
		input   CreateInput
	}{
		{name: "missing owner", input: CreateInput{Name: "Rules", Logic: LogicAnd,
			Conditions: []ConditionInput{validCondition}}},
		{name: "blank name", ownerID: uuid.New(), input: CreateInput{Name: "  ", Logic: LogicAnd,
			Conditions: []ConditionInput{validCondition}}},
		{name: "long name", ownerID: uuid.New(), input: CreateInput{Name: strings.Repeat("я", 151),
			Logic: LogicAnd, Conditions: []ConditionInput{validCondition}}},
		{name: "unknown logic", ownerID: uuid.New(), input: CreateInput{Name: "Rules", Logic: "XOR",
			Conditions: []ConditionInput{validCondition}}},
		{name: "empty conditions", ownerID: uuid.New(), input: CreateInput{Name: "Rules", Logic: LogicOr}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.ownerID, test.input)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("New() error = %v, want %v", err, ErrInvalid)
			}
		})
	}
}

func TestNewRejectsUnsupportedFieldOperatorAndValueCombinations(t *testing.T) {
	tests := []struct {
		name      string
		condition ConditionInput
	}{
		{name: "unknown field", condition: ConditionInput{
			Field: "popularity", Operator: OperatorEquals, Value: IntegerValue(10),
		}},
		{name: "title numeric comparison", condition: ConditionInput{
			Field: FieldTitle, Operator: OperatorGreaterThan, Value: TextValue("A"),
		}},
		{name: "blank title", condition: ConditionInput{
			Field: FieldTitle, Operator: OperatorContains, Value: TextValue("  "),
		}},
		{name: "UUID with text value", condition: ConditionInput{
			Field: FieldArtistID, Operator: OperatorEquals, Value: TextValue(uuid.NewString()),
		}},
		{name: "nil UUID", condition: ConditionInput{
			Field: FieldAlbumID, Operator: OperatorEquals, Value: UUIDValue(uuid.Nil),
		}},
		{name: "empty UUID list", condition: ConditionInput{
			Field: FieldGenreID, Operator: OperatorIn, Value: UUIDListValue(),
		}},
		{name: "duplicate UUID list", condition: duplicateUUIDCondition()},
		{name: "year outside range", condition: ConditionInput{
			Field: FieldReleaseYear, Operator: OperatorEquals, Value: IntegerValue(10000),
		}},
		{name: "reversed duration range", condition: ConditionInput{
			Field: FieldDurationMS, Operator: OperatorBetween, Value: IntegerRangeValue(5000, 1000),
		}},
		{name: "explicit contains", condition: ConditionInput{
			Field: FieldExplicit, Operator: OperatorContains, Value: BooleanValue(false),
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(uuid.New(), CreateInput{
				Name: "Rules", Logic: LogicAnd, Conditions: []ConditionInput{test.condition},
			})
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("New() error = %v, want %v", err, ErrInvalid)
			}
		})
	}
}

func duplicateUUIDCondition() ConditionInput {
	id := uuid.New()
	return ConditionInput{
		Field: FieldMoodID, Operator: OperatorIn, Value: UUIDListValue(id, id),
	}
}
