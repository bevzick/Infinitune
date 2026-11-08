package rules

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	maximumNameLength     = 150
	maximumTitleLength    = 200
	maximumUUIDListLength = 100
	minimumReleaseYear    = 1
	maximumReleaseYear    = 9999
	minimumDurationMS     = 1
	maximumDurationMS     = math.MaxInt32
)

var uuidFields = map[Field]struct{}{
	FieldArtistID: {},
	FieldAlbumID:  {},
	FieldGenreID:  {},
	FieldMoodID:   {},
}

func New(ownerID uuid.UUID, input CreateInput) (RuleSet, error) {
	if ownerID == uuid.Nil {
		return RuleSet{}, validationError("owner_id", "must be a UUID")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return RuleSet{}, validationError("name", "must not be blank")
	}
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > maximumNameLength {
		return RuleSet{}, validationError("name", "must contain at most 150 characters")
	}
	if input.Logic != LogicAnd && input.Logic != LogicOr {
		return RuleSet{}, validationError("logic", "must be AND or OR")
	}
	if len(input.Conditions) == 0 {
		return RuleSet{}, validationError("conditions", "must not be empty")
	}

	ruleSetID := uuid.New()
	conditions := make([]Condition, 0, len(input.Conditions))
	for index, condition := range input.Conditions {
		canonical, err := validateCondition(condition, index)
		if err != nil {
			return RuleSet{}, err
		}
		conditions = append(conditions, Condition{
			ID: uuid.New(), RuleSetID: ruleSetID,
			Field: canonical.Field, Operator: canonical.Operator,
			Value: canonical.Value, Position: index + 1,
		})
	}
	return RuleSet{
		ID: ruleSetID, OwnerID: ownerID, Name: name,
		Logic: input.Logic, Conditions: conditions,
	}, nil
}

func validateCondition(input ConditionInput, index int) (ConditionInput, error) {
	fieldPath := fmt.Sprintf("conditions[%d]", index)
	switch input.Field {
	case FieldTitle:
		if input.Operator != OperatorContains && input.Operator != OperatorEquals {
			return ConditionInput{}, validationError(fieldPath+".operator",
				"title supports contains or equals")
		}
		if input.Value.Kind != ValueText {
			return ConditionInput{}, valueKindError(fieldPath, ValueText)
		}
		input.Value.Text = strings.TrimSpace(input.Value.Text)
		if input.Value.Text == "" {
			return ConditionInput{}, validationError(fieldPath+".value", "must not be blank")
		}
		if !utf8.ValidString(input.Value.Text) ||
			utf8.RuneCountInString(input.Value.Text) > maximumTitleLength {
			return ConditionInput{}, validationError(fieldPath+".value",
				"must contain at most 200 characters")
		}
	case FieldReleaseYear:
		if err := validateIntegerCondition(input, fieldPath, minimumReleaseYear, maximumReleaseYear); err != nil {
			return ConditionInput{}, err
		}
	case FieldDurationMS:
		if err := validateIntegerCondition(input, fieldPath, minimumDurationMS, maximumDurationMS); err != nil {
			return ConditionInput{}, err
		}
	case FieldExplicit:
		if input.Operator != OperatorEquals {
			return ConditionInput{}, validationError(fieldPath+".operator",
				"explicit supports equals")
		}
		if input.Value.Kind != ValueBoolean {
			return ConditionInput{}, valueKindError(fieldPath, ValueBoolean)
		}
	default:
		if _, exists := uuidFields[input.Field]; !exists {
			return ConditionInput{}, validationError(fieldPath+".field", "is not supported")
		}
		if err := validateUUIDCondition(input, fieldPath); err != nil {
			return ConditionInput{}, err
		}
	}
	input.Value = canonicalValue(input.Value)
	return cloneConditionInput(input), nil
}

func validateUUIDCondition(input ConditionInput, fieldPath string) error {
	switch input.Operator {
	case OperatorEquals:
		if input.Value.Kind != ValueUUID {
			return valueKindError(fieldPath, ValueUUID)
		}
		if input.Value.UUID == uuid.Nil {
			return validationError(fieldPath+".value", "must be a UUID")
		}
	case OperatorIn:
		if input.Value.Kind != ValueUUIDList {
			return valueKindError(fieldPath, ValueUUIDList)
		}
		if len(input.Value.UUIDs) == 0 || len(input.Value.UUIDs) > maximumUUIDListLength {
			return validationError(fieldPath+".value", "must contain from 1 to 100 UUIDs")
		}
		seen := make(map[uuid.UUID]struct{}, len(input.Value.UUIDs))
		for _, id := range input.Value.UUIDs {
			if id == uuid.Nil {
				return validationError(fieldPath+".value", "must contain UUIDs")
			}
			if _, exists := seen[id]; exists {
				return validationError(fieldPath+".value", "must not contain duplicates")
			}
			seen[id] = struct{}{}
		}
	default:
		return validationError(fieldPath+".operator", "UUID fields support equals or in")
	}
	return nil
}

func validateIntegerCondition(
	input ConditionInput,
	fieldPath string,
	minimum int,
	maximum int,
) error {
	switch input.Operator {
	case OperatorEquals, OperatorGreaterThan, OperatorGreaterThanOrEqual,
		OperatorLessThan, OperatorLessThanOrEqual:
		if input.Value.Kind != ValueInteger {
			return valueKindError(fieldPath, ValueInteger)
		}
		if input.Value.Integer < minimum || input.Value.Integer > maximum {
			return validationError(fieldPath+".value",
				fmt.Sprintf("must be from %d to %d", minimum, maximum))
		}
	case OperatorBetween:
		if input.Value.Kind != ValueIntegerRange {
			return valueKindError(fieldPath, ValueIntegerRange)
		}
		value := input.Value.IntegerRange
		if value.Minimum < minimum || value.Maximum > maximum || value.Minimum > value.Maximum {
			return validationError(fieldPath+".value",
				fmt.Sprintf("range must be ordered and within %d to %d", minimum, maximum))
		}
	default:
		return validationError(fieldPath+".operator",
			"numeric fields support equals, comparisons, or between")
	}
	return nil
}

func cloneConditionInput(input ConditionInput) ConditionInput {
	input.Value.UUIDs = append([]uuid.UUID(nil), input.Value.UUIDs...)
	return input
}

func canonicalValue(value Value) Value {
	switch value.Kind {
	case ValueText:
		return TextValue(value.Text)
	case ValueUUID:
		return UUIDValue(value.UUID)
	case ValueUUIDList:
		return UUIDListValue(value.UUIDs...)
	case ValueInteger:
		return IntegerValue(value.Integer)
	case ValueIntegerRange:
		return IntegerRangeValue(value.IntegerRange.Minimum, value.IntegerRange.Maximum)
	case ValueBoolean:
		return BooleanValue(value.Boolean)
	default:
		return Value{Kind: value.Kind}
	}
}

func valueKindError(fieldPath string, expected ValueKind) error {
	return validationError(fieldPath+".value", "must use "+string(expected)+" value")
}

func validationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}
