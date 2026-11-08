package rules

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
)

type integerRangeDocument struct {
	Minimum int `json:"minimum"`
	Maximum int `json:"maximum"`
}

func EncodeValue(field Field, operator Operator, value Value) ([]byte, error) {
	canonical, err := validateCondition(ConditionInput{
		Field: field, Operator: operator, Value: value,
	}, 0)
	if err != nil {
		return nil, err
	}
	switch canonical.Value.Kind {
	case ValueText:
		return json.Marshal(canonical.Value.Text)
	case ValueUUID:
		return json.Marshal(canonical.Value.UUID.String())
	case ValueUUIDList:
		values := make([]string, 0, len(canonical.Value.UUIDs))
		for _, id := range canonical.Value.UUIDs {
			values = append(values, id.String())
		}
		return json.Marshal(values)
	case ValueInteger:
		return json.Marshal(canonical.Value.Integer)
	case ValueIntegerRange:
		return json.Marshal(integerRangeDocument{
			Minimum: canonical.Value.IntegerRange.Minimum,
			Maximum: canonical.Value.IntegerRange.Maximum,
		})
	case ValueBoolean:
		return json.Marshal(canonical.Value.Boolean)
	default:
		return nil, validationError("value", "has unsupported kind")
	}
}

func DecodeValue(field Field, operator Operator, document []byte) (Value, error) {
	kind, err := expectedValueKind(field, operator)
	if err != nil {
		return Value{}, err
	}
	var value Value
	switch kind {
	case ValueText:
		var decoded string
		if err = decodeDocument(document, &decoded); err == nil {
			value = TextValue(decoded)
		}
	case ValueUUID:
		var decoded string
		if err = decodeDocument(document, &decoded); err == nil {
			var id uuid.UUID
			id, err = uuid.Parse(decoded)
			value = UUIDValue(id)
		}
	case ValueUUIDList:
		var decoded []string
		if err = decodeDocument(document, &decoded); err == nil {
			ids := make([]uuid.UUID, 0, len(decoded))
			for _, raw := range decoded {
				id, parseErr := uuid.Parse(raw)
				if parseErr != nil {
					err = parseErr
					break
				}
				ids = append(ids, id)
			}
			value = UUIDListValue(ids...)
		}
	case ValueInteger:
		var decoded int
		if err = decodeDocument(document, &decoded); err == nil {
			value = IntegerValue(decoded)
		}
	case ValueIntegerRange:
		var decoded integerRangeDocument
		if err = decodeDocument(document, &decoded); err == nil {
			value = IntegerRangeValue(decoded.Minimum, decoded.Maximum)
		}
	case ValueBoolean:
		var decoded bool
		if err = decodeDocument(document, &decoded); err == nil {
			value = BooleanValue(decoded)
		}
	}
	if err != nil {
		return Value{}, fmt.Errorf("decode rule value: %w", err)
	}
	canonical, err := validateCondition(ConditionInput{
		Field: field, Operator: operator, Value: value,
	}, 0)
	if err != nil {
		return Value{}, err
	}
	return canonical.Value, nil
}

func expectedValueKind(field Field, operator Operator) (ValueKind, error) {
	switch field {
	case FieldTitle:
		return ValueText, nil
	case FieldArtistID, FieldAlbumID, FieldGenreID, FieldMoodID:
		if operator == OperatorIn {
			return ValueUUIDList, nil
		}
		return ValueUUID, nil
	case FieldReleaseYear, FieldDurationMS:
		if operator == OperatorBetween {
			return ValueIntegerRange, nil
		}
		return ValueInteger, nil
	case FieldExplicit:
		return ValueBoolean, nil
	default:
		return "", validationError("field", "is not supported")
	}
}

func decodeDocument(document []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("value must contain one JSON document")
	}
	return nil
}
