package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"strings"

	"k8s.io/kubernetes/pkg/cloudprovider"

	"github.com/golang/glog"
	"github.com/kubernetes/contrib/installer/k8sconfig"
	"github.com/kubernetes/contrib/installer/pkg/config"
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
)

type Bootstrap struct {
	Config        config.Configuration
	CloudProvider cloudprovider.Interface
}

func (b *Bootstrap) ReadConfig(configPath string) error {
	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error reading config file (%s): %v", configPath, err)
	}

	err = json.Unmarshal(configBytes, &b.Config)
	if err != nil {
		return fmt.Errorf("error parsing config file (%s): %v", configPath, err)
	}

	return nil
}

func main() {
	configPath := "/etc/kubernetes/config.yaml"
	flag.StringVar(&configPath, "config", configPath, "Path to kubernetes config file")

	roles := ""
	flag.StringVar(&roles, "roles", roles, "Roles to configure")

	resourcedir := ""
	flag.StringVar(&resourcedir, "resources", resourcedir, "Directory to scan for resources")

	validate := false
	flag.BoolVar(&validate, "validate", validate, "Perform validation only")

	flag.Set("alsologtostderr", "true")

	flag.Parse()

	/*

		var b Bootstrap
			err := b.ReadConfig(configPath)
			if err != nil {
				glog.Fatalf("unable to read configuration: %v", err)
			}
	*/

	config := fi.NewSimpleConfig()
	err := config.ReadYaml(configPath)
	if err != nil {
		glog.Fatalf("error reading configuration: %v", err)
	}

	c, err := fi.NewContext(config)
	if err != nil {
		glog.Fatalf("error building context: %v", err)
	}

	for _, role := range strings.Split(roles, ",") {
		c.AddRole(role)
	}
	if resourcedir != "" {
		c.AddResources(files.NewResourceDir(resourcedir))
	}

	bc := c.NewBuildContext()
	bc.Add(&k8sconfig.Kubernetes{})

	runMode := fi.ModeConfigure
	if validate {
		runMode = fi.ModeValidate
	}

	rc := c.NewRunContext(runMode)
	err = rc.Run()
	if err != nil {
		glog.Fatalf("error running configuration: %v", err)
	}

}
