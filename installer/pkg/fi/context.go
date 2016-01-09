package fi

import "github.com/golang/glog"

type Context struct {
	roles []string
	units []Unit
	state map[string]interface{}

	os    *OS
	cloud *Cloud
}

func NewContext() (*Context, error) {
	c := &Context{
		state: make(map[string]interface{}),
		os:    &OS{},
		cloud: &Cloud{},
	}

	err := c.os.init()
	if err != nil {
		return nil, err
	}

	err = c.cloud.init()
	if err != nil {
		return nil, err
	}

	return c, nil
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

func (c *Context) Cloud() *Cloud {
	return c.cloud
}

func (c *Context) Get(key string) string {
	glog.Warningf("variables not yet implemented: %v", key)
	return ""
}

func (c *Context) Add(unit Unit) {
	c.units = append(c.units, unit)
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
func (c *Context) Configure() error {
	for _, unit := range c.units {
		err := unit.Configure(c)
		if err != nil {
			return err
		}
	}
	return nil
}
