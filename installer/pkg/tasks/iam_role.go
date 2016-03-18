package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/golang/glog"
	"github.com/aws/aws-sdk-go/aws"
)

type IAMRoleRenderer interface {
	RenderIAMRole(actual, expected, changes *IAMRole) error
}

type IAMRole struct {
	ID                 *string
	Name               *string
	RolePolicyDocument Resource // "inline" policy
}

func (s *IAMRole) Prefix() string {
	return "IAMRole"
}

func (s *IAMRole) GetID() *string {
	return s.ID
}

func (e *IAMRole) find(c *Context) (*IAMRole, error) {
	cloud := c.Cloud

	request := &iam.GetRoleInput{RoleName: e.Name}

	response, err := cloud.IAM.GetRole(request)
	if err != nil {
		return nil, fmt.Errorf("error getting role: %v", err)
	}

	r := response.Role
	actual := &IAMRole{}
	actual.ID = r.RoleId
	actual.Name = r.RoleName
	glog.V(2).Infof("found matching IAMRole %q", *actual.ID)
	return actual, nil
}

func (e *IAMRole) Run(c *Context) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &IAMRole{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(IAMRoleRenderer)
	return target.RenderIAMRole(a, e, changes)
}

func (s *IAMRole) checkChanges(a, e, changes *IAMRole) error {
	if a != nil {
		if e.Name == nil {
			return MissingValueError("Name is required when creating IAMRole")
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderIAMRole(a, e, changes *IAMRole) error {
	if a == nil {
		glog.V(2).Infof("Creating IAMRole with Name:%q", *e.Name)

		policy, err := ResourceAsString(e.RolePolicyDocument)
		if err != nil {
			return fmt.Errorf("error rendering PolicyDocument: %v", err)
		}

		request := &iam.CreateRoleInput{}
		request.AssumeRolePolicyDocument = aws.String(policy)
		request.RoleName = e.Name

		response, err := t.cloud.IAM.CreateRole(request)
		if err != nil {
			return fmt.Errorf("error creating IAMRole: %v", err)
		}

		e.ID = response.Role.RoleId
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderIAMRole(a, e, changes *IAMRole) error {
	t.CreateVar(e)
	if a == nil {
		glog.V(2).Infof("Creating IAMRole with Name:%q", *e.Name)

		rolePolicyDocument, err := t.AddResource(e.RolePolicyDocument)
		if err != nil {
			return err
		}

		t.AddIAMCommand("create-role",
			"--role-name", *e.Name,
			"--assume-role-policy-document", rolePolicyDocument)
	}

	return nil
}
