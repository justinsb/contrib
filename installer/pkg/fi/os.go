package fi

import (
	"fmt"
	"os/exec"

	"github.com/golang/glog"
)

type OS struct {
	redhat bool
	debian bool
}

func (o *OS) init() error {
	lsbRelease, err := runLsbRelease("-i")
	if err != nil {
		return err
	}

	switch lsbRelease {
	case "Debian":
		o.debian = true
	default:
		panic("Unknown lsb_release " + lsbRelease)
	}

	return nil
}

func runLsbRelease(flag string) (string, error) {
	glog.V(2).Infof("Running lsb_release %s", flag)
	cmd := exec.Command("lsb_release", "-s", flag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error running lsb_release: %v: %s", err, string(output))
	}
	return string(output), nil
}

func (o *OS) IsRedhat() bool {
	return o.redhat
}

func (o *OS) IsDebian() bool {
	return o.debian
}
