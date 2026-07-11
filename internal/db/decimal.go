// Package db provides GORM-compatible glue types for exact decimal storage.
package db

import (
	"database/sql/driver"
	"fmt"

	"github.com/shopspring/decimal"
)

// Decimal wraps shopspring/decimal.Decimal so GORM can scan/value it directly
// against NUMERIC columns without ever passing through float64 (FR-020).
type Decimal struct {
	decimal.Decimal
}

// NewDecimal wraps a decimal.Decimal value.
func NewDecimal(d decimal.Decimal) Decimal {
	return Decimal{Decimal: d}
}

// NewDecimalFromString parses a string into a Decimal.
func NewDecimalFromString(s string) (Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Decimal{}, err
	}
	return Decimal{Decimal: d}, nil
}

// Scan implements sql.Scanner.
func (d *Decimal) Scan(value interface{}) error {
	if value == nil {
		d.Decimal = decimal.Zero
		return nil
	}
	switch v := value.(type) {
	case string:
		parsed, err := decimal.NewFromString(v)
		if err != nil {
			return err
		}
		d.Decimal = parsed
	case []byte:
		parsed, err := decimal.NewFromString(string(v))
		if err != nil {
			return err
		}
		d.Decimal = parsed
	case float64:
		d.Decimal = decimal.NewFromFloat(v)
	default:
		return fmt.Errorf("db.Decimal: unsupported scan type %T", value)
	}
	return nil
}

// Value implements driver.Valuer.
func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}

// MarshalJSON encodes the decimal as a JSON string (never a bare number),
// so wire-format cannot reintroduce float rounding (FR-020).
func (d Decimal) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON decodes a JSON string (or bare number, for tolerance) into a Decimal.
func (d *Decimal) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "null" || s == "" {
		d.Decimal = decimal.Zero
		return nil
	}
	parsed, err := decimal.NewFromString(s)
	if err != nil {
		return err
	}
	d.Decimal = parsed
	return nil
}

// NullDecimal is a nullable Decimal for columns like current_unit_cost that
// are NULL until a first value exists.
type NullDecimal struct {
	Decimal decimal.Decimal
	Valid   bool
}

// Scan implements sql.Scanner.
func (d *NullDecimal) Scan(value interface{}) error {
	if value == nil {
		d.Decimal = decimal.Zero
		d.Valid = false
		return nil
	}
	var inner Decimal
	if err := inner.Scan(value); err != nil {
		return err
	}
	d.Decimal = inner.Decimal
	d.Valid = true
	return nil
}

// Value implements driver.Valuer.
func (d NullDecimal) Value() (driver.Value, error) {
	if !d.Valid {
		return nil, nil
	}
	return d.Decimal.String(), nil
}

// MarshalJSON encodes null when not valid, else a decimal string.
func (d NullDecimal) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Decimal.String() + `"`), nil
}

// UnmarshalJSON decodes null or a decimal string/number into a NullDecimal.
func (d *NullDecimal) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		d.Decimal = decimal.Zero
		d.Valid = false
		return nil
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	parsed, err := decimal.NewFromString(s)
	if err != nil {
		return err
	}
	d.Decimal = parsed
	d.Valid = true
	return nil
}
