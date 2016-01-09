package fi

import (
	"bytes"
	"fmt"
)

type StringBuilder struct {
	b   bytes.Buffer
	err error
}

func (sb *StringBuilder) Append(s string) {
	sb.b.Write([]byte(s))
}

func (sb *StringBuilder) Appendf(format string, a ...interface{}) {
	s := fmt.Sprintf(format, a...)
	sb.b.Write([]byte(s))
}

func (s *StringBuilder) String() string {
	return string(s.b.Bytes())
}

func (s *StringBuilder) Error() error {
	return s.err
}
