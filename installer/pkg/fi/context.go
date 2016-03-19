package fi

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/golang/glog"
)

type Context struct {
	roles     []string
	state     map[string]interface{}

	os        *OS
	cloud     Cloud
	config    Config
	resources *ResourcesList

	root      *node
}

//type Context struct {
//	Cloud  *AWSCloud
//	Target Target
//}
//
//func NewContext(target Target, cloud *AWSCloud) *Context {
//	c := &Context{
//		Target: target,
//		Cloud:  cloud,
//	}
//	return c
//}


func NewContext(config Config, cloud Cloud) (*Context, error) {
	c := &Context{
		state:     make(map[string]interface{}),
		os:        &OS{},
		cloud:     cloud,
		resources: &ResourcesList{},
		config:    config,
	}

	c.root = &node{}

	err := c.os.init()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Context) NewRunContext(target Target, runMode RunMode) *RunContext {
	rc := &RunContext{
		Context: c,
		Target: target,
		node:    c.root,
		mode:    runMode,
	}
	return rc
}

func (c *Context) NewBuildContext() *BuildContext {
	bc := &BuildContext{
		Context: c,
		node:    c.root,
	}
	return bc
}

func (c *Context) AddRole(role string) {
	c.roles = append(c.roles, role)
}

func (c *Context) HasRole(role string) bool {
	for _, r := range c.roles {
		if r == role {
			return true
		}
	}
	return false
}

func (c *Context) OS() *OS {
	return c.os
}

func (c *Context) Cloud() Cloud {
	return c.cloud
}

func (c *Context) GetState(key string, builder func() (interface{}, error)) (interface{}, error) {
	v := c.state[key]
	if v == nil {
		var err error
		v, err = builder()
		if err != nil {
			return nil, err
		}
		c.state[key] = v
	}
	return v, nil
}

func (c *Context) newNode(unit Unit) *node {
	childNode := &node{
		unit: unit,
	}

	c.initializeNode(unit)

	return childNode
}

func (c *Context) initializeNode(unit Unit) {
	systemUnit, ok := unit.(SystemUnitInterface)
	if ok {
		if systemUnit.IsSystemUnit() {
			return
		}
	}

	unitValue := reflect.ValueOf(unit)

	if unitValue.Kind() == reflect.Ptr {
		unitValue = unitValue.Elem()
	}

	unitType := unitValue.Type()

	for i := 0; i < unitValue.NumField(); i++ {
		field := unitType.Field(i)

		if field.Anonymous {
			// Embedded field (struct, most likely)
			continue
		}

		fieldName := field.Name
		fieldType := field.Type
		fieldValue := unitValue.Field(i)

		fieldKey := fieldName
		glog.Warning("TODO: Check field id tag")

		switch fieldType.Kind() {

		case reflect.String:
			{
				v := fieldValue.String()
				if v != "" {
					// Honor values directly set
					continue
				}
				s, found := c.config.Get(fieldKey)
				if found {
					fieldValue.SetString(s)
				}
			}

		case reflect.Int:
			{
				v := fieldValue.Int()
				if v != 0 {
					// Honor values directly set
					continue
				}
				s, found := c.config.Get(fieldKey)
				if found {
					var err error
					v, err = strconv.ParseInt(s, 10, 64)
					if err != nil {
						panic("Unexpected error parsing config value: " + s + " for " + fieldName)
					}
					fieldValue.SetInt(v)
				}
			}

		case reflect.Bool:
			{
				v := fieldValue.Bool()
				if v != false {
					// Honor values directly set
					continue
				}
				s, found := c.config.Get(fieldKey)
				if found {
					var err error
					v, err = strconv.ParseBool(s)
					if err != nil {
						panic("Unexpected error parsing config value: " + s + " for " + fieldName)
					}
					fieldValue.SetBool(v)
				}
			}

		case reflect.Interface:
		case reflect.Slice:
		case reflect.Ptr:
		case reflect.Map:
			glog.Warningf("Ignoring field type: %v in %v::%v", fieldType.Kind(), unitType, field.Name)

		default:
			panic(fmt.Sprintf("Unhandled field type: %v in %v::%v", fieldType.Kind(), unitType, field.Name))
		}
	}
}

func (c *Context) Resource(key string) Resource {
	r, found := c.resources.Get(key)
	if !found {
		panic("Resource not found: " + key)
	}
	return r
}

func (c *Context) AddResources(r Resources) {
	c.resources.Add(r)
}
