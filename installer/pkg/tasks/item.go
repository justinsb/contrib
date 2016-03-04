package tasks

type Item interface {
	Run(context *Context) error
}
