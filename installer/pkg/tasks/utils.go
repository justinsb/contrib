package tasks

import (
	"reflect"

	"github.com/golang/glog"
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
