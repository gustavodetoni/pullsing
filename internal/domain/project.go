package domain

import "time"

type Project struct {
	ID        int64     `json:"id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewProject(key, name string) (Project, error) {
	if err := validateKey(key); err != nil {
		return Project{}, err
	}

	if err := validateName(name); err != nil {
		return Project{}, err
	}

	return Project{
		Key:  key,
		Name: name,
	}, nil
}
