package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"

	"k8s.io/kubernetes/pkg/cloudprovider"

	"github.com/golang/glog"
	"github.com/kubernetes/contrib/installer/k8sconfig"
	"github.com/kubernetes/contrib/installer/pkg/config"
	"github.com/kubernetes/contrib/installer/pkg/fi"
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
	configPath := "/etc/kubernetes/config.json"

	flag.StringVar(&configPath, "config", configPath, "Path to kubernetes config file")

	flag.Set("alsologtostderr", "true")

	flag.Parse()

	var b Bootstrap

	err := b.ReadConfig(configPath)
	if err != nil {
		glog.Fatalf("unable to read configuration: %v", err)
	}

	c, err := fi.NewContext()
	if err != nil {
		glog.Fatalf("error building context: %v", err)
	}

	k8sconfig.Add(c)

	err = c.Configure()
	if err != nil {
		glog.Fatalf("error running configuration: %v", err)
	}

}
