package fi

type Target interface {
	PutResource(resource Resource) (url string, hash string, err error)
}
