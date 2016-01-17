package fi

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
)

func HasContents(path string, contents []byte) (bool, error) {
	in, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("error opening file %q: %v", path, err)
	}
	defer in.Close()

	// TODO: Stream?  But probably not, because we should only be doing this for smallish files
	inContents, err := ioutil.ReadAll(in)
	if err != nil {
		return false, fmt.Errorf("error reading file %q: %v", path, err)
	}

	return bytes.Equal(inContents, contents), nil
}
