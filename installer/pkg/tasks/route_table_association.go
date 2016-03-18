package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
)

type RouteTableAssociationRenderer interface {
	RenderRouteTableAssociation(actual, expected, changes *RouteTableAssociation) error
}

type RouteTableAssociation struct {
	fi.SimpleUnit

	ID           *string
	RouteTable   *RouteTable
	RouteTableID *string
	Subnet       *Subnet
	SubnetID     *string
}

func (s *RouteTableAssociation) Prefix() string {
	return "RouteTableAssociation"
}

func (s *RouteTableAssociation) GetID() *string {
	return s.ID
}

func (e *RouteTableAssociation) find(c *fi.RunContext) (*RouteTableAssociation, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	routeTableID := e.RouteTableID
	if routeTableID == nil && e.RouteTable != nil {
		routeTableID = e.RouteTable.ID
	}

	subnetID := e.SubnetID
	if subnetID == nil && e.Subnet != nil {
		subnetID = e.Subnet.ID
	}

	if routeTableID == nil || subnetID == nil {
		return nil, nil
	}

	filters := cloud.BuildFilters()
	filters = append(filters, fi.NewEC2Filter("association.subnet-id", *subnetID))
	request := &ec2.DescribeRouteTablesInput{
		RouteTableIds: []*string{routeTableID},
		Filters:       filters,
	}

	response, err := cloud.EC2.DescribeRouteTables(request)
	if err != nil {
		return nil, fmt.Errorf("error listing RouteTables: %v", err)
	}
	if response == nil || len(response.RouteTables) == 0 {
		return nil, nil
	} else {
		if len(response.RouteTables) != 1 {
			glog.Fatalf("found multiple RouteTables matching tags")
		}
		rt := response.RouteTables[0]
		for _, rta := range rt.Associations {
			actual := &RouteTableAssociation{}
			actual.ID = rta.RouteTableAssociationId
			actual.RouteTableID = rta.RouteTableId
			actual.SubnetID = rta.SubnetId
			glog.V(2).Infof("found matching RouteTableAssociation %q", *actual.ID)
			return actual, nil
		}
	}

	return nil, nil
}

func (e *RouteTableAssociation) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &RouteTableAssociation{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(RouteTableAssociationRenderer)
	return target.RenderRouteTableAssociation(a, e, changes)
}

func (s *RouteTableAssociation) checkChanges(a, e, changes *RouteTableAssociation) error {
	if a != nil {
		if changes.RouteTableID != nil {
			return InvalidChangeError("Cannot change RouteTableAssociation RouteTable", changes.RouteTableID, e.RouteTableID)
		}
		if changes.SubnetID != nil {
			return InvalidChangeError("Cannot change RouteTableAssociation Subnet", changes.SubnetID, e.SubnetID)
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderRouteTableAssociation(a, e, changes *RouteTableAssociation) error {
	if a == nil {
		subnetID := e.SubnetID
		if subnetID == nil && e.Subnet != nil {
			subnetID = e.Subnet.ID
		}
		if subnetID == nil {
			return MissingValueError("Must specify Subnet for RouteTableAssociation create")
		}

		routeTableID := e.RouteTableID
		if routeTableID == nil && e.RouteTable != nil {
			routeTableID = e.RouteTable.ID
		}
		if routeTableID == nil {
			return MissingValueError("Must specify RouteTable for RouteTableAssociation create")
		}

		glog.V(2).Infof("Creating RouteTableAssociation with RouteTable:%q Subnet:%q", *routeTableID, *subnetID)

		request := &ec2.AssociateRouteTableInput{}
		request.SubnetId = subnetID
		request.RouteTableId = routeTableID

		response, err := t.cloud.EC2.AssociateRouteTable(request)
		if err != nil {
			return fmt.Errorf("error creating RouteTableAssociation: %v", err)
		}

		e.ID = response.AssociationId
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderRouteTableAssociation(a, e, changes *RouteTableAssociation) error {
	t.CreateVar(e)
	if a == nil {
		subnetID := StringValue(e.SubnetID)
		if subnetID == "" {
			subnetID = t.ReadVar(e.Subnet)
		}

		routeTableID := StringValue(e.RouteTableID)
		if routeTableID == "" {
			routeTableID = t.ReadVar(e.RouteTable)
		}

		glog.V(2).Infof("Creating RouteTableAssociation with RouteTable:%q Subnet:%q", routeTableID, subnetID)

		t.AddEC2Command("associate-route-table", "--route-table-id", routeTableID, "--subnet-id", subnetID)
	} else {
		t.AddAssignment(e, StringValue(a.ID))
	}

	return nil
	//return output.AddAWSTags(cloud.Tags(), r, "route-table-association")
}
