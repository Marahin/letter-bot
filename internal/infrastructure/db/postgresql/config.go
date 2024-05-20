package postgresql

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

type Specification struct {
	Host     string `required:"true" default:"db"`
	Port     int    `required:"true" default:"5432"`
	User     string `required:"true" default:"postgres"`
	Password string `required:"true" default:"postgres"`
	Name     string `required:"true" default:"name"`
}

func Dsn() string {
	var s Specification

	envconfig.MustProcess("database", &s)

	return fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		s.Host, s.Port, s.User, s.Password, s.Name)
}
