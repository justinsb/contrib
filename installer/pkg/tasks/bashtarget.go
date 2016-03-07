package tasks

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

type BashTarget struct {
	// TODO: Remove cloud
	cloud    *AWSCloud
	commands []*BashCommand

	ec2Args              []string
	autoscalingArgs      []string
	iamArgs              []string
	vars                 map[HasId]*BashVar
	prefixCounts         map[string]int
	resourcePrefixCounts map[string]int
}

func NewBashTarget(cloud *AWSCloud) *BashTarget {
	b := &BashTarget{cloud: cloud}
	b.ec2Args = []string{"aws", "ec2"}
	b.autoscalingArgs = []string{"aws", "autoscaling"}
	b.iamArgs = []string{"aws", "iam"}
	b.vars = make(map[HasId]*BashVar)
	b.prefixCounts = make(map[string]int)
	b.resourcePrefixCounts = make(map[string]int)
	return b
}

type BashVar struct {
	name        string
	staticValue *string
}

func (t *BashTarget) CreateVar(v HasId) *BashVar {
	bv, found := t.vars[v]
	if found {
		glog.Fatal("Attempt to create variable twice for ", v)
	}
	bv = &BashVar{}
	prefix := strings.ToUpper(v.Prefix())
	n := t.prefixCounts[prefix]
	n++
	t.prefixCounts[prefix] = n

	bv.name = prefix + "_" + strconv.Itoa(n)
	t.vars[v] = bv
	return bv
}

type BashCommand struct {
	parent   *BashTarget
	args     []string
	assignTo *BashVar
}

func (c *BashCommand) AssignTo(s HasId) *BashCommand {
	bv := c.parent.vars[s]
	if bv == nil {
		glog.Fatal("no variable assigned to ", s)
	}
	c.assignTo = bv
	return c
}

func (c *BashCommand) DebugDump() {
	if c.assignTo != nil {
		glog.Info("CMD: ", c.assignTo.name, "=`", c.args, "`")
	} else {
		glog.Info("CMD: ", c.args)
	}
}

func (c *BashCommand) PrintShellCommand(w io.Writer) error {
	var buf bytes.Buffer

	if c.assignTo != nil {
		buf.WriteString(c.assignTo.name)
		buf.WriteString("=`")
	}

	for i, arg := range c.args {
		if i != 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(arg)
	}

	if c.assignTo != nil {
		buf.WriteString("`")
	}

	buf.WriteString("\n")

	_, err := buf.WriteTo(w)
	return err
}

func (t *BashTarget) ReadVar(s HasId) string {
	bv := t.vars[s]
	if bv == nil {
		glog.Fatal("no variable assigned to ", s)
	}

	// TODO: Escaping?
	return "${" + bv.name + "}"
}

func (t *BashTarget) DebugDump() {
	for _, cmd := range t.commands {
		cmd.DebugDump()
	}
}

func (t *BashTarget) PrintShellCommands(w io.Writer) error {
	var header bytes.Buffer
	header.WriteString("#!/bin/bash\n")
	header.WriteString("set -ex\n\n")
	header.WriteString(". ./helpers\n\n")

	for k, v := range t.cloud.EnvVars() {
		header.WriteString("export " + k + "=" + bashQuoteString(v) + "\n")
	}

	_, err := header.WriteTo(w)
	if err != nil {
		return err
	}

	for _, cmd := range t.commands {
		err = cmd.PrintShellCommand(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *BashTarget) AddEC2Command(args ...string) *BashCommand {
	cmd := &BashCommand{parent: t}
	cmd.args = t.ec2Args
	cmd.args = append(cmd.args, args...)

	return t.AddCommand(cmd)
}

func (t *BashTarget) AddAutoscalingCommand(args ...string) *BashCommand {
	cmd := &BashCommand{parent: t}
	cmd.args = t.autoscalingArgs
	cmd.args = append(cmd.args, args...)

	return t.AddCommand(cmd)
}

func (t *BashTarget) AddS3Command(region string, args ...string) *BashCommand {
	cmd := &BashCommand{parent: t}
	cmd.args = []string{"aws", "s3", "--region", region}
	cmd.args = append(cmd.args, args...)

	return t.AddCommand(cmd)
}

func (t *BashTarget) AddS3APICommand(region string, args ...string) *BashCommand {
	cmd := &BashCommand{parent: t}
	cmd.args = []string{"aws", "s3api", "--region", region}
	cmd.args = append(cmd.args, args...)

	return t.AddCommand(cmd)
}

func (t *BashTarget) AddIAMCommand(args ...string) *BashCommand {
	cmd := &BashCommand{parent: t}
	cmd.args = t.iamArgs
	cmd.args = append(cmd.args, args...)

	return t.AddCommand(cmd)
}

func bashQuoteString(s string) string {
	// TODO: Escaping
	var quoted bytes.Buffer
	for _, c := range s {
		switch c {
		case '"':
			quoted.WriteString("\\\"")
		default:
			quoted.WriteString(string(c))
		}
	}

	return "\"" + string(quoted.Bytes()) + "\""
}

func (t *BashTarget) AddAWSTags(expected map[string]string, s HasId, resourceType string) error {
	resourceId, exists := t.FindValue(s)
	var missing map[string]string
	if exists {
		actual, err := t.cloud.GetTags(resourceId, resourceType)
		if err != nil {
			return fmt.Errorf("unexpected error fetchin tags for resource: %v", err)
		}

		missing = map[string]string{}
		for k, v := range expected {
			actualValue, found := actual[k]
			if found && actualValue == v {
				continue
			}
			missing[k] = v
		}
	} else {
		missing = expected
	}

	for name, value := range missing {
		cmd := &BashCommand{}
		cmd.args = []string{"add-tag", t.ReadVar(s), bashQuoteString(name), bashQuoteString(value)}
		t.AddCommand(cmd)
	}

	return nil
}

func (t *BashTarget) AddCommand(cmd *BashCommand) *BashCommand {
	t.commands = append(t.commands, cmd)

	return cmd
}

func (t *BashTarget) AddAssignment(h HasId, value string) {
	bv := t.vars[h]
	if bv == nil {
		glog.Fatal("no variable assigned to ", h)
	}

	cmd := &BashCommand{}
	assign := bv.name + "=" + bashQuoteString(value)
	cmd.args = []string{assign}
	t.AddCommand(cmd)

	bv.staticValue = &value
}

func (t *BashTarget) FindValue(h HasId) (string, bool) {
	bv := t.vars[h]
	if bv == nil {
		glog.Fatal("no variable assigned to ", h)
	}

	if bv.staticValue == nil {
		return "", false
	}
	return *bv.staticValue, true
}

func (t *BashTarget) generateDynamicPath(prefix string) string {
	basePath := "resources"
	n := t.resourcePrefixCounts[prefix]
	n++
	t.resourcePrefixCounts[prefix] = n

	name := prefix + "_" + strconv.Itoa(n)
	p := path.Join(basePath, name)
	return p
}

func (t *BashTarget) AddResource(resource Resource) (string, error) {
	dynamicResource, ok := resource.(DynamicResource)
	if ok {
		path := t.generateDynamicPath(dynamicResource.Prefix())
		f, err := os.Create(path)
		if err != nil {
			return "", err
		}
		defer func() {
			err := f.Close()
			if err != nil {
				glog.Warning("Error closing resource file", err)
			}
		}()

		err = dynamicResource.Write(f)
		if err != nil {
			return "", fmt.Errorf("error writing resource: %v", err)
		}

		return path, nil
	}

	switch r := resource.(type) {
	case *FileResource:
		return r.Path, nil
	default:
		log.Fatal("unknown resource type: ", r)
		return "", fmt.Errorf("unknown resource type: %v", r)
	}
}
