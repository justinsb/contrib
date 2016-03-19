package fi

type Target interface {
	PutResource(key string, resource Resource) (url string, hash string, err error)
}
