/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package connection

import (
	"encoding/base64"
	"fmt"
	"github.com/go-playground/validator/v10"
	"net/url"
)

// password length is set to match edgex security bootstrapper length
const passwordLength = 33

// Config represents the configuration for connection
type Config struct {
	Host     string `validate:"required"`
	Port     int    `validate:"required,gte=0,lte=65535"`
	User     string `validate:"required"`
	Password string `validate:"required,checkPassword"`
	DBName   string `validate:"required"`
	SSLMode  string `validate:"required"`
}

// ConnectionString constructs connection string using pq scheme
func (c Config) ConnectionString() (string, error) {
	err := c.validate()
	if err != nil {
		return "", err
	}

	u := url.URL{
		Scheme:   "postgresql",
		User:     url.UserPassword(c.User, c.Password),
		Host:     fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:     c.DBName,
		RawQuery: url.Values{"sslmode": []string{c.SSLMode}}.Encode(),
	}

	return u.String(), nil
}

func (c Config) validate() error {
	validate := validator.New()
	err := validate.RegisterValidation("checkPassword", c.checkPassword)
	if err != nil {
		return err
	}

	if err := validate.Struct(c); err != nil {
		return err
	}

	return nil
}

func (c Config) checkPassword(fl validator.FieldLevel) bool {
	password, err := base64.StdEncoding.DecodeString(fl.Field().String())
	if err != nil {
		return false
	}

	if len(password) != passwordLength {
		return false
	}
	return true
}
