package fi

import "github.com/golang/glog"

type Cloud struct {
	aws bool
	gce bool
}

func (c *Cloud) init() error {
	// For EC2, check /sys/hypervisor/uuid, then check 169.254.169.254
	//	cat /sys/hypervisor/uuid
	//	ec21884e-23e4-dcf2-d27e-4495fedb2abd

	// curl http://169.254.169.254/
	glog.Warning("Cloud detection hard-coded")

	return nil
}

func (c *Cloud) IsAWS() bool {
	return true
}

func (c *Cloud) IsGCE() bool {
	return false
}

func (c *Cloud) IsVagrant() bool {
	return false
}
