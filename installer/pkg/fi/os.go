package fi

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/golang/glog"
)

type OS struct {
	redhat bool
	debian bool
	ubuntu bool

	codename string
}

func (o *OS) init() error {
	distributor, err := runLsbRelease("-i")
	if err != nil {
		return err
	}

	switch distributor {
	case "Debian":
		o.debian = true
	case "Ubuntu":
		o.ubuntu = true
	default:
		panic(fmt.Sprintf("Unknown lsb_release distributor %q", distributor))
	}

	o.codename, err = runLsbRelease("-c")
	if err != nil {
		return err
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

	s := string(output)
	// Remove any trailing '\n'
	s = strings.TrimSpace(s)
	return s, nil
}

func (o *OS) IsRedhat() bool {
	return o.redhat
}

func (o *OS) IsDebian() bool {
	return o.debian
}

func (o *OS) IsUbuntu() bool {
	return o.ubuntu
}

func (o *OS) IsUbuntuVivid() bool {
	return o.IsUbuntu() && o.codename == "vivid"
}
