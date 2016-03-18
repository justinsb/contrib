package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type InstanceRenderer interface {
	RenderInstance(actual, expected, changes *Instance) error
}

type Instance struct {
	ID               *string
	InstanceCommonConfig

	Subnet           *Subnet
	PrivateIPAddress *string

	Name             *string
	Tags             map[string]string
}

func (s *Instance) Prefix() string {
	return "Instance"
}

func (s *Instance) GetID() *string {
	return s.ID
}

func (e *Instance) find(c *Context) (*Instance, error) {
	cloud := c.Cloud

	filters := cloud.BuildFilters()
	filters = append(filters, newEc2Filter("tag:Name", *e.Name))
	filters = append(filters, newEc2Filter("instance-state-name", "pending", "running", "stopping", "stopped"))
	request := &ec2.DescribeInstancesInput{
		Filters: filters,
	}

	response, err := cloud.EC2.DescribeInstances(request)
	if err != nil {
		return nil, fmt.Errorf("error listing instances: %v", err)
	}

	instances := []*ec2.Instance{}
	if response == nil {
		for _, reservation := range response.Reservations {
			for _, instance := range reservation.Instances {
				instances = append(instances, instance)
			}
		}
	}

	if len(instances) == 0 {
		return nil, nil
	}

	if len(instances) != 1 {
		return nil, fmt.Errorf("found multiple Instances with name: %s", e.Name)
	}

	glog.V(2).Info("found existing instance")
	i := instances[0]
	actual := &Instance{}
	actual.ID = i.InstanceId
	return actual, nil
}

func (e *Instance) Run(c *Context) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &Instance{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(InstanceRenderer)
	return target.RenderInstance(a, e, changes)
}

func (s *Instance) checkChanges(a, e, changes *Instance) error {
	if a != nil {
		if e.Name == nil {
			return MissingValueError("Name is required when creating Instance")
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderInstance(a, e, changes *Instance) error {
	if a == nil {
		glog.V(2).Infof("Creating Instance with Name:%q", *e.Name)

		request := &ec2.RunInstancesInput{}
		request.ImageId = e.ImageID
		request.InstanceType = e.InstanceType
		if e.SSHKey != nil {
			request.KeyName = e.SSHKey.Name
		}
		securityGroupIDs := []*string{}
		for _, sg := range e.SecurityGroups {
			securityGroupIDs = append(securityGroupIDs, sg.ID)
		}
		request.SecurityGroups = securityGroupIDs
		request.NetworkInterfaces = []*ec2.InstanceNetworkInterfaceSpecification{
			{
				DeviceIndex:aws.Int64(0),
				AssociatePublicIpAddress:e.AssociatePublicIP,
				SubnetId:e.Subnet.ID,
			},
		}
		request.PrivateIpAddress = e.PrivateIPAddress

		if e.BlockDeviceMappings != nil {
			request.BlockDeviceMappings = []*ec2.BlockDeviceMapping{}
			for _, b := range e.BlockDeviceMappings {
				request.BlockDeviceMappings = append(request.BlockDeviceMappings, b.ToEC2())
			}
		}

		if e.UserData != nil {
			d, err := ResourceAsString(e.UserData)
			if err != nil {
				return fmt.Errorf("error rendering Instance UserData: %v", err)
			}
			request.UserData = aws.String(d)
		}
		if e.IAMInstanceProfile != nil {
			request.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
				Name: e.IAMInstanceProfile.Name,
			}
		}

		response, err := t.cloud.EC2.RunInstances(request)
		if err != nil {
			return fmt.Errorf("error creating Instance: %v", err)
		}

		e.ID = response.Instances[0].InstanceId

		tags := make(map[string]string)
		for k, v := range e.Tags {
			tags[k] = v
		}
		if e.Name != nil {
			tags["Name"] = *e.Name
		}
		return t.cloud.CreateTags("instance", *e.ID, tags)
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderInstance(a, e, changes *Instance) error {
	t.CreateVar(e)
	if a == nil {
		glog.V(2).Infof("Creating Instance with Name:%q", *e.Name)

		args := []string{"run-instances"}
		args = append(args, e.buildEC2CreateArgs(t)...)

		if e.Subnet != nil {
			args = append(args, "--subnet-id", t.ReadVar(e.Subnet))
		}
		if e.PrivateIPAddress != nil {
			args = append(args, "--private-ip-address", *e.PrivateIPAddress)
		}

		args = append(args, "--query", "Instances[0].InstanceId")

		t.AddEC2Command(args...).AssignTo(e)
	} else {
		t.AddAssignment(e, aws.StringValue(a.ID))
	}

	//tags := cloud.Tags()
	tags := make(map[string]string)
	tags["Name"] = *e.Name

	if e.Tags != nil {
		for k, v := range e.Tags {
			tags[k] = v
		}
	}
	//return t.AddAWSTags(tags, i, "instance")

	return nil
}

/*
func (i *Instance) Destroy(cloud *AWSCloud, output *BashTarget) error {
	existing, err := i.findExisting(cloud)
	if err != nil {
		return err
	}

	if existing != nil {
		glog.V(2).Info("Found instance; will delete: ", i)
		args := []string{"terminate-instances"}
		args = append(args, "--instance-ids", aws.StringValue(existing.InstanceId))

		output.AddEC2Command(args...).AssignTo(i)
	}

	return nil
}
*/
