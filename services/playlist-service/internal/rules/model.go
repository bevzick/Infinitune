package rules

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalid = errors.New("invalid rule set")

type LogicOperator string

const (
	LogicAnd LogicOperator = "AND"
	LogicOr  LogicOperator = "OR"
)

type Field string

const (
	FieldTitle       Field = "title"
	FieldArtistID    Field = "artist_id"
	FieldAlbumID     Field = "album_id"
	FieldGenreID     Field = "genre_id"
	FieldMoodID      Field = "mood_id"
	FieldReleaseYear Field = "release_year"
	FieldDurationMS  Field = "duration_ms"
	FieldExplicit    Field = "explicit"
)

type Operator string

const (
	OperatorEquals             Operator = "equals"
	OperatorContains           Operator = "contains"
	OperatorIn                 Operator = "in"
	OperatorGreaterThan        Operator = "greater_than"
	OperatorGreaterThanOrEqual Operator = "greater_than_or_equal"
	OperatorLessThan           Operator = "less_than"
	OperatorLessThanOrEqual    Operator = "less_than_or_equal"
	OperatorBetween            Operator = "between"
)

type ValueKind string

const (
	ValueText         ValueKind = "text"
	ValueUUID         ValueKind = "uuid"
	ValueUUIDList     ValueKind = "uuid_list"
	ValueInteger      ValueKind = "integer"
	ValueIntegerRange ValueKind = "integer_range"
	ValueBoolean      ValueKind = "boolean"
)

type IntegerRange struct {
	Minimum int
	Maximum int
}

type Value struct {
	Kind         ValueKind
	Text         string
	UUID         uuid.UUID
	UUIDs        []uuid.UUID
	Integer      int
	IntegerRange IntegerRange
	Boolean      bool
}

func TextValue(value string) Value {
	return Value{Kind: ValueText, Text: value}
}

func UUIDValue(value uuid.UUID) Value {
	return Value{Kind: ValueUUID, UUID: value}
}

func UUIDListValue(values ...uuid.UUID) Value {
	return Value{Kind: ValueUUIDList, UUIDs: append([]uuid.UUID(nil), values...)}
}

func IntegerValue(value int) Value {
	return Value{Kind: ValueInteger, Integer: value}
}

func IntegerRangeValue(minimum, maximum int) Value {
	return Value{
		Kind:         ValueIntegerRange,
		IntegerRange: IntegerRange{Minimum: minimum, Maximum: maximum},
	}
}

func BooleanValue(value bool) Value {
	return Value{Kind: ValueBoolean, Boolean: value}
}

type Condition struct {
	ID        uuid.UUID
	RuleSetID uuid.UUID
	Field     Field
	Operator  Operator
	Value     Value
	Position  int
}

type RuleSet struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	Name       string
	Logic      LogicOperator
	Conditions []Condition
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ConditionInput struct {
	Field    Field
	Operator Operator
	Value    Value
}

type CreateInput struct {
	Name       string
	Logic      LogicOperator
	Conditions []ConditionInput
}

type ValidationError struct {
	Field  string
	Reason string
}

func (err *ValidationError) Error() string {
	return err.Field + ": " + err.Reason
}

func (err *ValidationError) Unwrap() error {
	return ErrInvalid
}
