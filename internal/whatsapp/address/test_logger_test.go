package address

import "github.com/rs/zerolog"

func zerologNop() zerolog.Logger {
	return zerolog.Nop()
}
