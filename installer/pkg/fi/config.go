package fi

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode"
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

func (c *SimpleConfig) Get(fieldName string) (string, bool) {
	key := toUnderscore(fieldName)
	v, found := c.config[key]
	return v, found
}

func (c *SimpleConfig) Read(file string) error {
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
		v := strings.TrimSpace(tokens[1])

		c.config[k] = v
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading config file %q: %v", file, err)
	}

	return nil
}
