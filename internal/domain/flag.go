package domain

import (
	"errors"
	"time"
)

var ErrUnsupportedFlagType = errors.New("unsupported flag type")

const FlagTypeBool = "bool"

type Flag struct {
	ID            int64      `json:"id"`
	EnvironmentID int64      `json:"environment_id"`
	Key           string     `json:"key"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Type          string     `json:"type"`
	Enabled       bool       `json:"enabled"`
	BoolValue     bool       `json:"bool_value"`
	Revision      int64      `json:"revision"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ArchivedAt    *time.Time `json:"archived_at,omitempty"`
}

func NewBoolFlag(environmentID int64, key, name, description string, enabled, boolValue bool) (Flag, error) {
	if environmentID <= 0 {
		return Flag{}, ErrInvalidKey
	}

	if err := validateKey(key); err != nil {
		return Flag{}, err
	}

	if err := validateName(name); err != nil {
		return Flag{}, err
	}

	return Flag{
		EnvironmentID: environmentID,
		Key:           key,
		Name:          name,
		Description:   description,
		Type:          FlagTypeBool,
		Enabled:       enabled,
		BoolValue:     boolValue,
	}, nil
}
