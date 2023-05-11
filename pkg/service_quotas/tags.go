package servicequotas

import (
	"regexp"
	"strings"

	logging "github.com/sirupsen/logrus"
)

var log = logging.WithFields(logging.Fields{})

var invalidLabelCharactersRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

// ToPrometheusNamingFormat modifies string `s` to conform with the Prom naming
// conventions
func ToPrometheusNamingFormat(s string) string {
	return toSnakeCase(invalidLabelCharactersRE.ReplaceAllString(s, "_"))
}

func toSnakeCase(s string) string {
	snake := matchAllCap.ReplaceAllString(s, "${1}_${2}")
	return strings.ToLower(snake)
}
