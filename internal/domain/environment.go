package domain

import "time"

type Environment struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Revision  int64     `json:"revision"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewEnvironment(projectID int64, key, name string) (Environment, error) {
	if projectID <= 0 {
		return Environment{}, ErrInvalidKey
	}

	if err := validateKey(key); err != nil {
		return Environment{}, err
	}

	if err := validateName(name); err != nil {
		return Environment{}, err
	}

	return Environment{
		ProjectID: projectID,
		Key:       key,
		Name:      name,
	}, nil
}
