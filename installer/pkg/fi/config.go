package fi

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/golang/glog"
)

type Config interface {
	Get(fieldName string) (string, bool)
}

type SimpleConfig struct {
	config map[string]string
}

var _ Config = &SimpleConfig{}

func NewSimpleConfig() *SimpleConfig {
	return &SimpleConfig{
		config: make(map[string]string),
	}
}

// TODO: Move to class (along with toArgName)?
func toUnderscore(s string) string {
	var b bytes.Buffer
	for i, r := range s {
		if unicode.IsUpper(r) {
			// New word
			if i != 0 {
				b.WriteRune('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}

	return string(b.Bytes())
}

func fromYamlKey(s string) string {
	var b bytes.Buffer
	toUpper := true
	for _, r := range s {
		if r == '-' || r == '_' {
			toUpper = true
			continue
		}

		if toUpper {
			r = unicode.ToUpper(r)
			toUpper = false
		}
		b.WriteRune(r)
	}

	return string(b.Bytes())
}

func fromYamlValue(s string) string {
	if s == "" {
		return s
	}

	if s[0] == '\'' && s[len(s)-1] == '\'' {
		s = s[1 : len(s)-1]
	} else if s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}

	return s
}

func (c *SimpleConfig) Get(fieldName string) (string, bool) {
	v, found := c.config[fieldName]
	return v, found
}

func (c *SimpleConfig) ReadYaml(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("error opening config file %q: %v", file, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		tokens := strings.SplitN(line, ":", 2)
		if len(tokens) < 2 {
			return fmt.Errorf("cannot parse config line %q in file %q", line, file)
		}

		k := strings.TrimSpace(tokens[0])
		k = fromYamlKey(k)

		v := strings.TrimSpace(tokens[1])
		v = fromYamlValue(v)

		glog.V(2).Infof("Read configuration value: %s=%s", k, v)

		c.config[k] = v
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading config file %q: %v", file, err)
	}

	return nil
}
