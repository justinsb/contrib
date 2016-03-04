package tasks

type Context struct {
	Cloud  *AWSCloud
	Target Target
}

func NewContext(target Target, cloud *AWSCloud) *Context {
	c := &Context{
		Target: target,
		Cloud:  cloud,
	}
	return c
}
