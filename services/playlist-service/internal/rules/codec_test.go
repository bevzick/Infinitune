package rules

import (
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestRuleValueCodecRoundTrip(t *testing.T) {
	idOne := uuid.New()
	idTwo := uuid.New()
	tests := []struct {
		name     string
		field    Field
		operator Operator
		value    Value
	}{
		{name: "text", field: FieldTitle, operator: OperatorContains, value: TextValue("ambient")},
		{name: "UUID", field: FieldArtistID, operator: OperatorEquals, value: UUIDValue(idOne)},
		{name: "UUID list", field: FieldGenreID, operator: OperatorIn, value: UUIDListValue(idOne, idTwo)},
		{name: "integer", field: FieldReleaseYear, operator: OperatorGreaterThanOrEqual, value: IntegerValue(2020)},
		{name: "range", field: FieldDurationMS, operator: OperatorBetween, value: IntegerRangeValue(60000, 240000)},
		{name: "boolean false", field: FieldExplicit, operator: OperatorEquals, value: BooleanValue(false)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, err := EncodeValue(test.field, test.operator, test.value)
			if err != nil {
				t.Fatalf("EncodeValue() error = %v", err)
			}
			decoded, err := DecodeValue(test.field, test.operator, document)
			if err != nil {
				t.Fatalf("DecodeValue() error = %v", err)
			}
			if !reflect.DeepEqual(decoded, test.value) {
				t.Fatalf("DecodeValue() = %+v, want %+v", decoded, test.value)
			}
		})
	}
}

func TestRuleValueCodecRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name     string
		field    Field
		operator Operator
		document string
	}{
		{name: "wrong JSON type", field: FieldExplicit, operator: OperatorEquals, document: `"false"`},
		{name: "invalid UUID", field: FieldArtistID, operator: OperatorEquals, document: `"invalid"`},
		{name: "unknown range field", field: FieldDurationMS, operator: OperatorBetween,
			document: `{"minimum":1000,"maximum":2000,"other":3}`},
		{name: "multiple documents", field: FieldReleaseYear, operator: OperatorEquals, document: `2020 2021`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeValue(test.field, test.operator, []byte(test.document)); err == nil {
				t.Fatal("DecodeValue() error = nil")
			}
		})
	}
}

func TestReplacePreservesRuleSetIdentity(t *testing.T) {
	ruleSetID := uuid.New()
	ownerID := uuid.New()
	replaced, err := Replace(ruleSetID, ownerID, CreateInput{
		Name: "Updated", Logic: LogicOr, Conditions: []ConditionInput{{
			Field: FieldExplicit, Operator: OperatorEquals, Value: BooleanValue(true),
		}},
	})
	if err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	if replaced.ID != ruleSetID || replaced.OwnerID != ownerID ||
		len(replaced.Conditions) != 1 || replaced.Conditions[0].RuleSetID != ruleSetID {
		t.Fatalf("Replace() = %+v", replaced)
	}
	if _, err = Replace(uuid.Nil, ownerID, CreateInput{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Replace(nil) error = %v, want %v", err, ErrInvalid)
	}
}
