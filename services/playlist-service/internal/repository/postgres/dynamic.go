package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repository *PlaylistRepository) CreateDynamic(
	ctx context.Context,
	value playlist.Playlist,
	ruleSet rules.RuleSet,
) (playlist.DynamicPlaylist, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return playlist.DynamicPlaylist{}, fmt.Errorf("begin create dynamic playlist transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	createdRules, err := insertRuleSet(ctx, tx, ruleSet)
	if err != nil {
		return playlist.DynamicPlaylist{}, err
	}
	createdPlaylist, err := scanPlaylist(tx.QueryRow(ctx, `
		INSERT INTO playlist.playlists (
			id, owner_id, name, description, playlist_type, visibility, rule_set_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id, owner_id, name, description, playlist_type, visibility,
			rule_set_id, created_at, updated_at
	`, value.ID, value.OwnerID, value.Name, value.Description, value.Type,
		value.Visibility, value.RuleSetID))
	if err != nil {
		return playlist.DynamicPlaylist{}, fmt.Errorf("insert dynamic playlist: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return playlist.DynamicPlaylist{}, fmt.Errorf("commit create dynamic playlist transaction: %w", err)
	}
	return playlist.DynamicPlaylist{Playlist: createdPlaylist, RuleSet: createdRules}, nil
}

func (repository *PlaylistRepository) RuleSetByID(
	ctx context.Context,
	id uuid.UUID,
) (rules.RuleSet, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT
			rule_set.id, rule_set.owner_id, rule_set.name,
			rule_set.logic_operator, rule_set.created_at, rule_set.updated_at,
			condition.id, condition.rule_set_id, condition.field_name,
			condition.operator, condition.value, condition.position
		FROM rule_engine.rule_sets AS rule_set
		JOIN rule_engine.rule_conditions AS condition
			ON condition.rule_set_id = rule_set.id
		WHERE rule_set.id = $1
		ORDER BY condition.position
	`, id)
	if err != nil {
		return rules.RuleSet{}, fmt.Errorf("query rule set by id: %w", err)
	}
	defer rows.Close()

	var result rules.RuleSet
	for rows.Next() {
		var condition rules.Condition
		var logic string
		var field string
		var operator string
		var document []byte
		if err = rows.Scan(
			&result.ID,
			&result.OwnerID,
			&result.Name,
			&logic,
			&result.CreatedAt,
			&result.UpdatedAt,
			&condition.ID,
			&condition.RuleSetID,
			&field,
			&operator,
			&document,
			&condition.Position,
		); err != nil {
			return rules.RuleSet{}, fmt.Errorf("scan rule set: %w", err)
		}
		result.Logic = rules.LogicOperator(logic)
		condition.Field = rules.Field(field)
		condition.Operator = rules.Operator(operator)
		condition.Value, err = rules.DecodeValue(condition.Field, condition.Operator, document)
		if err != nil {
			return rules.RuleSet{}, fmt.Errorf("decode stored rule condition: %w", err)
		}
		result.Conditions = append(result.Conditions, condition)
	}
	if err = rows.Err(); err != nil {
		return rules.RuleSet{}, fmt.Errorf("iterate rule set: %w", err)
	}
	if result.ID == uuid.Nil {
		return rules.RuleSet{}, rules.ErrNotFound
	}
	return result, nil
}

func (repository *PlaylistRepository) ReplaceRuleSet(
	ctx context.Context,
	replacement rules.RuleSet,
) (rules.RuleSet, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return rules.RuleSet{}, fmt.Errorf("begin replace rule set transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated, err := scanRuleSet(tx.QueryRow(ctx, `
		UPDATE rule_engine.rule_sets
		SET name = $3, logic_operator = $4, updated_at = now()
		WHERE id = $1 AND owner_id = $2
		RETURNING id, owner_id, name, logic_operator, created_at, updated_at
	`, replacement.ID, replacement.OwnerID, replacement.Name, replacement.Logic))
	if errors.Is(err, pgx.ErrNoRows) {
		return rules.RuleSet{}, rules.ErrNotFound
	}
	if err != nil {
		return rules.RuleSet{}, fmt.Errorf("update rule set: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		DELETE FROM rule_engine.rule_conditions WHERE rule_set_id = $1
	`, replacement.ID); err != nil {
		return rules.RuleSet{}, fmt.Errorf("delete old rule conditions: %w", err)
	}
	if err = insertConditions(ctx, tx, replacement.Conditions); err != nil {
		return rules.RuleSet{}, err
	}
	updated.Conditions = replacement.Conditions
	if _, err = tx.Exec(ctx, `
		UPDATE playlist.playlists
		SET updated_at = now()
		WHERE rule_set_id = $1 AND owner_id = $2
	`, replacement.ID, replacement.OwnerID); err != nil {
		return rules.RuleSet{}, fmt.Errorf("touch dynamic playlist: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return rules.RuleSet{}, fmt.Errorf("commit replace rule set transaction: %w", err)
	}
	return updated, nil
}

func insertRuleSet(
	ctx context.Context,
	tx pgx.Tx,
	ruleSet rules.RuleSet,
) (rules.RuleSet, error) {
	var result rules.RuleSet
	var logic string
	err := tx.QueryRow(ctx, `
		INSERT INTO rule_engine.rule_sets (id, owner_id, name, logic_operator)
		VALUES ($1, $2, $3, $4)
		RETURNING id, owner_id, name, logic_operator, created_at, updated_at
	`, ruleSet.ID, ruleSet.OwnerID, ruleSet.Name, ruleSet.Logic).Scan(
		&result.ID,
		&result.OwnerID,
		&result.Name,
		&logic,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if err != nil {
		return rules.RuleSet{}, fmt.Errorf("insert rule set: %w", err)
	}
	result.Logic = rules.LogicOperator(logic)
	if err = insertConditions(ctx, tx, ruleSet.Conditions); err != nil {
		return rules.RuleSet{}, err
	}
	result.Conditions = ruleSet.Conditions
	return result, nil
}

func insertConditions(ctx context.Context, tx pgx.Tx, conditions []rules.Condition) error {
	for _, condition := range conditions {
		value, err := rules.EncodeValue(condition.Field, condition.Operator, condition.Value)
		if err != nil {
			return fmt.Errorf("encode rule condition value: %w", err)
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO rule_engine.rule_conditions (
				id, rule_set_id, field_name, operator, value, position
			)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, condition.ID, condition.RuleSetID, condition.Field, condition.Operator,
			value, condition.Position); err != nil {
			return fmt.Errorf("insert rule condition: %w", err)
		}
	}
	return nil
}

func scanRuleSet(row pgx.Row) (rules.RuleSet, error) {
	var result rules.RuleSet
	var logic string
	err := row.Scan(
		&result.ID,
		&result.OwnerID,
		&result.Name,
		&logic,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	result.Logic = rules.LogicOperator(logic)
	return result, err
}

var _ playlist.DynamicStore = (*PlaylistRepository)(nil)
