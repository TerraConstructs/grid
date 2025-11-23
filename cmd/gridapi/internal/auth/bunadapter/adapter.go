package bunadapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"github.com/uptrace/bun"
)

// Forked from github.com/msales/casbin-bun-adapter at v1.0.7 and updated to drop the
// hard-coded Postgres schema qualifier so the adapter works with schema-less
// table names (e.g. SQLite and Postgres public schema).
// removed ID and rnd

// Filter represents adapter filter.
type Filter struct {
	P []string
	G []string
}

// Adapter represents the github.com/uptrace/bun adapter for policy storage.
type Adapter struct {
	db       *bun.DB
	filtered bool
}

// NewAdapter creates new Adapter by using bun's database connection.
// Expects DB table to be created in database.
func NewAdapter(db *bun.DB) (*Adapter, error) {
	return &Adapter{db: db}, nil
}

// LoadPolicy loads policy from the database.
func (a *Adapter) LoadPolicy(model model.Model) error {
	var rules []*CasbinRule

	if err := a.db.NewSelect().Model(&rules).Scan(context.Background()); err != nil {
		return fmt.Errorf("failed to load policy from adapter db: %w", err)
	}

	for _, r := range rules {
		values, lastNonEmpty := r.toValueSlice()
		if lastNonEmpty == -1 {
			continue // skip empty rule
		}
		_ = model.AddPolicy(r.Ptype, r.Ptype, values[:lastNonEmpty+1])
	}

	a.filtered = false

	return nil
}

// SavePolicy saves policy to the database removing any policies already present.
func (a *Adapter) SavePolicy(model model.Model) error {
	rules := a.extractRules(model)

	if err := a.save(true, rules...); err != nil {
		return fmt.Errorf("failed to save policy to adapter db: %w", err)
	}

	return nil
}

// AddPolicy adds adapter policy rule to the database.
func (a *Adapter) AddPolicy(_ string, ptype string, rule []string) error {
	r := newCasbinRule(ptype, rule)

	if err := a.save(false, r); err != nil {
		return fmt.Errorf("failed to add adapter policy rule: %w", err)
	}

	return nil
}

// AddPolicies adds policy rules to the database.
func (a *Adapter) AddPolicies(_ string, ptype string, rules [][]string) error {
	casbinRules := make([]*CasbinRule, 0, len(rules))
	for _, rule := range rules {
		casbinRules = append(casbinRules, newCasbinRule(ptype, rule))
	}

	if err := a.save(false, casbinRules...); err != nil {
		return fmt.Errorf("failed to add policy rules: %w", err)
	}

	return nil
}

// RemovePolicy removes adapter policy rule from the database.
func (a *Adapter) RemovePolicy(_ string, ptype string, rule []string) error {
	r := newCasbinRule(ptype, rule)

	if err := a.delete(r); err != nil {
		return fmt.Errorf("failed to remove adapter policy rule: %w", err)
	}

	return nil
}

// RemovePolicies removes policy rules from the database.
func (a *Adapter) RemovePolicies(_ string, ptype string, rules [][]string) error {
	var casbinRules []*CasbinRule
	for _, rule := range rules {
		casbinRules = append(casbinRules, newCasbinRule(ptype, rule))
	}

	if err := a.delete(casbinRules...); err != nil {
		return fmt.Errorf("failed to remove policy rules: %w", err)
	}

	return nil
}

// RemoveFilteredPolicy removes policy rules that match the filter from the database.
func (a *Adapter) RemoveFilteredPolicy(_ string, ptype string, fieldIndex int, fieldValues ...string) error {
	query := a.db.NewDelete().Model((*CasbinRule)(nil)).Where("ptype = ?", ptype)

	idx := fieldIndex + len(fieldValues)
	if fieldIndex <= 0 && idx > 0 && fieldValues[0-fieldIndex] != "" {
		query = query.Where("v0 = ?", fieldValues[0-fieldIndex])
	}
	if fieldIndex <= 1 && idx > 1 && fieldValues[1-fieldIndex] != "" {
		query = query.Where("v1 = ?", fieldValues[1-fieldIndex])
	}
	if fieldIndex <= 2 && idx > 2 && fieldValues[2-fieldIndex] != "" {
		query = query.Where("v2 = ?", fieldValues[2-fieldIndex])
	}
	if fieldIndex <= 3 && idx > 3 && fieldValues[3-fieldIndex] != "" {
		query = query.Where("v3 = ?", fieldValues[3-fieldIndex])
	}
	if fieldIndex <= 4 && idx > 4 && fieldValues[4-fieldIndex] != "" {
		query = query.Where("v4 = ?", fieldValues[4-fieldIndex])
	}
	if fieldIndex <= 5 && idx > 5 && fieldValues[5-fieldIndex] != "" {
		query = query.Where("v5 = ?", fieldValues[5-fieldIndex])
	}

	if _, err := query.Exec(context.Background()); err != nil {
		return fmt.Errorf("failed to remove filtered adapter policy: %w", err)
	}

	return nil
}

// LoadFilteredPolicy loads only policies that match the filter.
func (a *Adapter) LoadFilteredPolicy(model model.Model, filter any) error {
	f, ok := filter.(*Filter)
	if !ok {
		return fmt.Errorf("invalid filter type: %T", filter)
	}

	var policies []*CasbinRule
	query := a.db.NewSelect().Model(&policies)
	query, err := a.buildQuery(query, f.P)
	if err != nil {
		return err
	}

	var groupings []*CasbinRule
	groupQuery := a.db.NewSelect().Model(&groupings).Where("ptype = ?", "g")
	groupQuery, err = a.buildQuery(groupQuery, f.G)
	if err != nil {
		return err
	}

	if err := query.Scan(context.Background()); err != nil {
		return fmt.Errorf("failed to load filtered policy: %w", err)
	}
	if err := groupQuery.Scan(context.Background()); err != nil {
		return fmt.Errorf("failed to load filtered groupings: %w", err)
	}

	for _, line := range policies {
		_ = persist.LoadPolicyLine(line.String(), model)
	}

	for _, line := range groupings {
		_ = persist.LoadPolicyLine(line.String(), model)
	}

	a.filtered = true

	return nil
}

// IsFiltered returns true if the loaded policy has been filtered.
func (a *Adapter) IsFiltered() bool {
	return a.filtered
}

// UpdatePolicy updates adapter policy rule from the database.
// This is part of the Auto-Save feature.
func (a *Adapter) UpdatePolicy(sec string, ptype string, oldRule, newPolicy []string) error {
	return a.UpdatePolicies(sec, ptype, [][]string{oldRule}, [][]string{newPolicy})
}

// UpdatePolicies updates some policy rules to the database.
func (a *Adapter) UpdatePolicies(_ string, ptype string, oldRules, newRules [][]string) error {
	oldLines := make([]*CasbinRule, 0, len(oldRules))
	newLines := make([]*CasbinRule, 0, len(newRules))

	for _, rule := range oldRules {
		oldLines = append(oldLines, newCasbinRule(ptype, rule))
	}

	for _, rule := range newRules {
		newLines = append(newLines, newCasbinRule(ptype, rule))
	}

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}

	for i, line := range oldLines {
		q := tx.NewUpdate().Model(newLines[i])
		qb := q.QueryBuilder()
		line.QueryWhereGroup(qb)
		_, err = q.Exec(context.Background())
		if err != nil {
			_ = tx.Rollback()

			return err
		}
	}

	return tx.Commit()
}

// UpdateFilteredPolicies updates some policy rules in the database.
func (a *Adapter) UpdateFilteredPolicies(_ string, ptype string, newRules [][]string, fieldIndex int, fieldValues ...string) ([][]string, error) {
	line := &CasbinRule{}

	line.Ptype = ptype
	if fieldIndex <= 0 && 0 < fieldIndex+len(fieldValues) {
		line.V0 = fieldValues[0-fieldIndex]
	}
	if fieldIndex <= 1 && 1 < fieldIndex+len(fieldValues) {
		line.V1 = fieldValues[1-fieldIndex]
	}
	if fieldIndex <= 2 && 2 < fieldIndex+len(fieldValues) {
		line.V2 = fieldValues[2-fieldIndex]
	}
	if fieldIndex <= 3 && 3 < fieldIndex+len(fieldValues) {
		line.V3 = fieldValues[3-fieldIndex]
	}
	if fieldIndex <= 4 && 4 < fieldIndex+len(fieldValues) {
		line.V4 = fieldValues[4-fieldIndex]
	}
	if fieldIndex <= 5 && 5 < fieldIndex+len(fieldValues) {
		line.V5 = fieldValues[5-fieldIndex]
	}

	newP := make([]CasbinRule, 0, len(newRules))
	for _, nr := range newRules {
		newP = append(newP, *(newCasbinRule(ptype, nr)))
	}

	oldP := make([]CasbinRule, 0)
	oldP = append(oldP, *line)

	err := a.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		for i := range newP {
			q := tx.NewDelete().Model(&oldP)
			qb := q.QueryBuilder()
			line.QueryWhereGroup(qb)
			_, err := q.Returning("*").Exec(ctx)
			if err != nil {
				return err
			}

			_, err = tx.NewInsert().Model(&newP[i]).On("CONFLICT DO NOTHING").Exec(ctx)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// return deleted rules
	oldPolicies := make([][]string, 0)
	for _, v := range oldP {
		oldPolicy := v.toStringPolicy()
		oldPolicies = append(oldPolicies, oldPolicy)
	}
	return oldPolicies, nil
}

// Close closes adapter database connection.
func (a *Adapter) Close() error {
	return a.db.Close()
}

func (a *Adapter) extractRules(model model.Model) []*CasbinRule {
	var casbinRules []*CasbinRule

	for ptype, assertion := range model["p"] {
		for _, rule := range assertion.Policy {
			casbinRules = append(casbinRules, newCasbinRule(ptype, rule))
		}
	}

	for ptype, assertion := range model["g"] {
		for _, rule := range assertion.Policy {
			casbinRules = append(casbinRules, newCasbinRule(ptype, rule))
		}
	}

	return casbinRules
}

func (a *Adapter) save(truncate bool, lines ...*CasbinRule) error {
	return a.db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		if truncate {
			_, err := tx.NewTruncateTable().Model((*CasbinRule)(nil)).Exec(context.Background())
			if err != nil {
				return err
			}
		}

		for _, line := range lines {
			_, err := tx.NewInsert().Model(line).On("CONFLICT DO NOTHING").Exec(context.Background())
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (a *Adapter) delete(lines ...*CasbinRule) error {
	if len(lines) == 0 {
		return nil
	}

	delQuery := a.db.NewDelete().Model((*CasbinRule)(nil))
	delQuery.QueryBuilder().WhereGroup("AND", func(q bun.QueryBuilder) bun.QueryBuilder {
		return q.WhereGroup("OR", func(q bun.QueryBuilder) bun.QueryBuilder {
			for _, line := range lines {
				line.QueryWhereGroup(q)
			}
			return q
		})
	})
	_, err := delQuery.Exec(context.Background())
	if err != nil {
		return err
	}

	return nil
}

func (a *Adapter) buildQuery(query *bun.SelectQuery, values []string) (*bun.SelectQuery, error) {
	for ind, v := range values {
		if v == "" {
			continue
		}
		switch ind {
		case 0:
			query = query.Where("v0 = ?", v)
		case 1:
			query = query.Where("v1 = ?", v)
		case 2:
			query = query.Where("v2 = ?", v)
		case 3:
			query = query.Where("v3 = ?", v)
		case 4:
			query = query.Where("v4 = ?", v)
		case 5:
			query = query.Where("v5 = ?", v)
		default:
			return nil, fmt.Errorf("filter has more values than expected, should not exceed 6 values")
		}
	}
	return query, nil
}

// CasbinRule represents adapter rule in Casbin.
type CasbinRule struct {
	bun.BaseModel `bun:"table:casbin_rules,alias:cr"`

	// Removed ID field,
	// defined a composite primary key on all fields instead.
	// strongly typed fields to varchar with length limits
	Ptype string `bun:",pk,type:varchar(100),notnull"` // Policy type: 'p' (policy), 'g' (grouping/role)
	V0    string `bun:",pk,type:varchar(255)"`         // Role name (policies) or user ID (groupings)
	V1    string `bun:",pk,type:varchar(255)"`         // Object type or role name
	V2    string `bun:",pk,type:varchar(255)"`         // Action
	V3    string `bun:",pk,type:varchar(255)"`         // Scope expression (go-bexpr string)
	V4    string `bun:",pk,type:varchar(255)"`         // Effect (allow/deny)
	V5    string `bun:",pk,type:varchar(255)"`         // Reserved
}

func newCasbinRule(ptype string, rule []string) *CasbinRule {
	line := &CasbinRule{Ptype: ptype}

	l := len(rule)
	if l > 0 {
		line.V0 = rule[0]
	}
	if l > 1 {
		line.V1 = rule[1]
	}
	if l > 2 {
		line.V2 = rule[2]
	}
	if l > 3 {
		line.V3 = rule[3]
	}
	if l > 4 {
		line.V4 = rule[4]
	}
	if l > 5 {
		line.V5 = rule[5]
	}

	return line
}

func (r *CasbinRule) String() string {
	const prefixLine = ", "
	var sb strings.Builder

	sb.Grow(
		len(r.Ptype) +
			len(r.V0) + len(r.V1) + len(r.V2) +
			len(r.V3) + len(r.V4) + len(r.V5),
	)

	sb.WriteString(r.Ptype)

	// Build the values array to determine the last non-empty field
	values := []string{r.V0, r.V1, r.V2, r.V3, r.V4, r.V5}
	lastNonEmpty := -1
	for i := len(values) - 1; i >= 0; i-- {
		if values[i] != "" {
			lastNonEmpty = i
			break
		}
	}

	// Write all fields up to and including the last non-empty one
	// This preserves empty fields in the middle
	for i := 0; i <= lastNonEmpty; i++ {
		sb.WriteString(prefixLine)
		sb.WriteString(values[i])
	}

	return sb.String()
}

// QueryWhereGroup extends query builder with another OR group. Group contains all non-empty fields of the CasbinRule.
func (r *CasbinRule) QueryWhereGroup(q bun.QueryBuilder) bun.QueryBuilder {
	q.WhereGroup("OR", func(q bun.QueryBuilder) bun.QueryBuilder {
		q = q.Where("ptype = ?", r.Ptype)
		if r.V0 != "" {
			q = q.Where("v0 = ?", r.V0)
		}
		if r.V1 != "" {
			q = q.Where("v1 = ?", r.V1)
		}
		if r.V2 != "" {
			q = q.Where("v2 = ?", r.V2)
		}
		if r.V3 != "" {
			q = q.Where("v3 = ?", r.V3)
		}
		if r.V4 != "" {
			q = q.Where("v4 = ?", r.V4)
		}
		if r.V5 != "" {
			q = q.Where("v5 = ?", r.V5)
		}
		return q
	})
	return q
}

func (r *CasbinRule) toValueSlice() ([]string, int) {
	values := []string{r.V0, r.V1, r.V2, r.V3, r.V4, r.V5}
	lastNonEmpty := -1
	for i := len(values) - 1; i >= 0; i-- {
		if values[i] != "" {
			lastNonEmpty = i
			break
		}
	}
	return values, lastNonEmpty
}

func (r *CasbinRule) toStringPolicy() []string {
	// Build the values array to determine the last non-empty field
	values, lastNonEmpty := r.toValueSlice()

	// Build policy slice with ptype + all fields up to the last non-empty one
	// This preserves empty fields in the middle
	policy := make([]string, 0, lastNonEmpty+2) // +2 for ptype and 0-indexed to count conversion

	if r.Ptype != "" {
		policy = append(policy, r.Ptype)
	}

	// Add all fields up to and including the last non-empty one
	for i := 0; i <= lastNonEmpty; i++ {
		policy = append(policy, values[i])
	}

	return policy
}
