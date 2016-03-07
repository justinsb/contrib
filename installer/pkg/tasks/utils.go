package tasks

import (
	"reflect"

	"github.com/golang/glog"
)

func BuildChanges(a, e, changes interface{}) bool {
	changed := false

	va := reflect.ValueOf(a)
	ve := reflect.ValueOf(e)
	vc := reflect.ValueOf(changes)

	va = va.Elem()
	ve = ve.Elem()
	vc = vc.Elem()

	t := vc.Type()
	if t != ve.Type() {
		panic("mismatched types in BuildChanges")
	}
	if t != va.Type() {
		panic("mismatched types in BuildChanges")
	}

	for i := 0; i < va.NumField(); i++ {
		fva := va.Field(i)
		fve := ve.Field(i)

		if fve.IsNil() {
			// No expected value means 'don't change'
			continue
		}

		if reflect.DeepEqual(fva.Interface(), fve.Interface()) {
			continue
		}

		glog.V(8).Infof("Field changed %q %q %q", t.Field(i).Name, fva.Interface(), fve.Interface())
		changed = true
		vc.Field(i).Set(fva)
	}

	return changed
}

func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
