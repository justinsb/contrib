package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/aws"

	"github.com/golang/glog"
	"github.com/kubernetes/contrib/installer/pkg/config"
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

func (b *Bootstrap) BuildCloudProvider() error {
	cloudProvider, err := cloudprovider.GetCloudProvider(b.Config.CloudProvider, strings.NewReader(b.Config.CloudProviderConfig))
	if err != nil {
		return fmt.Errorf("error initializing cloud provider: %v", err)
	}
	if cloudProvider == nil {
		return fmt.Errorf("cloud provider not found (%s)", b.Config.CloudProvider)
	}
	b.CloudProvider = cloudProvider
	return nil
}

func (b *Bootstrap) MountMasterVolume() error {
	// We could discover the volume using metadata, but probably easier just to rely on it being present in config
	volumes, ok := b.CloudProvider.(aws_cloud.Volumes)
	if !ok {
		return fmt.Errorf("cloud provider does not support volumes (%s)", b.Config.CloudProvider)
	}

	instanceName := "" // Indicates "self"
	volumeName := b.Config.MasterVolume
	readOnly := false
	device := "/dev/xvdb"
	device, err := volumes.AttachDisk(instanceName, volumeName, readOnly, device)
	if err != nil {
		// TODO: This is retryable
		return fmt.Errorf("error mounting volume (%s): %v", volumeName, readOnly)
	}

	/*
		fstype := "ext4"
		options := []string{}

		util.SafeFormatAndMount(device, mountpoint, fstype, options)
	*/

	return nil
}

func (b *Bootstrap) ConfigureMasterRoute() error {
	// Once we have it insert our entry into the routing table
	routes, ok := b.CloudProvider.Routes()
	if !ok {
		// TODO: Should this just be a warning?
		return fmt.Errorf("cloudprovider %s does not support routes", b.Config.CloudProvider)
	}

	// TODO: Is there a better way to get "my name"?
	instances, ok := b.CloudProvider.Instances()
	if !ok {
		return fmt.Errorf("cloudprovider %s does not support instance info", b.Config.CloudProvider)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("error getting hostname: %v", err)
	}

	myNodeName, err := instances.CurrentNodeName(hostname)
	if err != nil {
		return fmt.Errorf("unable to determine current node name: %v", err)
	}

	route := &cloudprovider.Route{
		TargetInstance:  myNodeName,
		DestinationCIDR: b.Config.MasterCIDR,
	}

	nameHint := "" // TODO: master?
	err = routes.CreateRoute(b.Config.ClusterID, nameHint, route)
	if err != nil {
		return fmt.Errorf("error creating route: %v", err)
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

	err = b.BuildCloudProvider()
	if err != nil {
		glog.Fatalf("unable to build cloud provider: %v", err)
	}

	err = b.MountMasterVolume()
	if err != nil {
		glog.Fatalf("unable to mount master volume: %v", err)
	}

	err = b.ConfigureMasterRoute()
	if err != nil {
		glog.Fatalf("unable to configure master route: %v", err)
	}

	// Proceed!
}
