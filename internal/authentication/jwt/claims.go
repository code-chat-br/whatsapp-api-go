package jwt

import gojwt "github.com/golang-jwt/jwt/v5"

type InstanceClaims struct {
	InstanceName string `json:"instanceName"`
	gojwt.RegisteredClaims
}
