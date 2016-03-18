package tasks

import (
	"reflect"
	crypto_rand "crypto/rand"

	"github.com/golang/glog"
	"encoding/json"
	"fmt"
	"encoding/base64"
	"bytes"
)

func BuildChanges(a, e, changes interface{}) bool {
	changed := false

	ve := reflect.ValueOf(e)
	vc := reflect.ValueOf(changes)

	ve = ve.Elem()
	vc = vc.Elem()

	t := vc.Type()
	if t != ve.Type() {
		panic("mismatched types in BuildChanges")
	}

	va := reflect.ValueOf(a)
	aIsNil := false

	if va.IsNil() {
		aIsNil = true
	}
	if !aIsNil {
		va = va.Elem()

		if t != va.Type() {
			panic("mismatched types in BuildChanges")
		}
	}

	for i := 0; i < ve.NumField(); i++ {
		fve := ve.Field(i)
		if fve.Kind() == reflect.Ptr && fve.IsNil() {
			// No expected value means 'don't change'
			continue
		}

		if !aIsNil {
			fva := va.Field(i)

			if reflect.DeepEqual(fva.Interface(), fve.Interface()) {
				continue
			}

			glog.V(8).Infof("Field changed %q %q %q", t.Field(i).Name, fva.Interface(), fve.Interface())
		}
		changed = true
		vc.Field(i).Set(fve)
	}

	return changed
}

func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func BuildString(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("error marshalling: %v", err)
	}
	return string(data)
}


func RandomToken(length int) string {
	// This is supposed to be the same algorithm as the old bash algorithm
	// KUBELET_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
	// KUBE_PROXY_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)

	for {
		buffer := make([]byte, length * 4)
		_, err := crypto_rand.Read(buffer)
		if err != nil {
			glog.Fatalf("error generating random token: %v", err)
		}
		s := base64.StdEncoding.EncodeToString(buffer)
		var trimmed bytes.Buffer
		for _, c := range s {
			switch c {
			case '=', '+', '/':
				continue
			default:
				trimmed.WriteRune(c)
			}
		}

		s = string(trimmed.Bytes())
		if len(s) >= length {
			return s[0:length]
		}
	}
}

var templateDir = "templates"

type HasId interface {
	Prefix() string
	GetID() *string
}

