package postgresql

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

// Specification is the database connection, read from DATABASE_*.
type Specification struct {
	Host     string `required:"true" default:"db"`
	Port     int    `required:"true" default:"5432"`
	User     string `required:"true" default:"postgres"`
	Password string `required:"true" default:"postgres"`
	Name     string `required:"true" default:"name"`
	SSL      string `default:"disable"`
}

// LoadConfig reads DATABASE_* from the environment.
func LoadConfig() (Specification, error) {
	var s Specification
	if err := envconfig.Process("database", &s); err != nil {
		return Specification{}, err
	}
	return s, nil
}

// DSN is the libpq connection string of s.
func (s Specification) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		s.Host, s.Port, s.User, s.Password, s.Name, s.SSL)
}

// Dsn reads DATABASE_* and panics when the environment is invalid.
func Dsn() string {
	var s Specification
	envconfig.MustProcess("database", &s)
	return s.DSN()
}
