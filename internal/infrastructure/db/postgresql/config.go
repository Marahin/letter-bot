package postgresql

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

type Specification struct {
	Host     string `required:"true"`
	Port     int    `required:"true"`
	User     string `required:"true"`
	Password string `required:"true"`
	Name     string `required:"true"`
}

func Dsn() string {
	var s Specification

	envconfig.MustProcess("database", &s)

	return fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		s.Host, s.Port, s.User, s.Password, s.Name)
}
