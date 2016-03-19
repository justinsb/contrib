package tasks

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/fi/filestore"
	"os"
)

type BashTarget struct {
	// TODO: Remove cloud
	cloud                *fi.AWSCloud
	filestore            filestore.FileStore
	commands             []*BashCommand
	ec2Args              []string
	autoscalingArgs      []string
	iamArgs              []string
	vars                 map[string]*BashVar
	prefixCounts         map[string]int
	resourcePrefixCounts map[string]int
}

var _ fi.Target = &BashTarget{}

func NewBashTarget(cloud *fi.AWSCloud, filestore filestore.FileStore) *BashTarget {
	b := &BashTarget{cloud: cloud, filestore: filestore}
	b.ec2Args = []string{"aws", "ec2"}
	b.autoscalingArgs = []string{"aws", "autoscaling"}
	b.iamArgs = []string{"aws", "iam"}
	b.vars = make(map[string]*BashVar)
	b.prefixCounts = make(map[string]int)
	b.resourcePrefixCounts = make(map[string]int)
	return b
}

type BashVar struct {
	name        string
	staticValue *string
}

func getVariablePrefix(v fi.Unit) string {
	name := fi.GetTypeName(v)
	name = strings.ToUpper(name)
	return name
}

func getKey(v fi.Unit) string {
	return v.Path()
}

func (t *BashTarget) CreateVar(v fi.Unit) *BashVar {
	key := getKey(v)
	bv, found := t.vars[key]
	if found {
		glog.Fatalf("Attempt to create variable twice for %q: %v", key, v)
	}
	bv = &BashVar{}
	prefix := getVariablePrefix(v)
	n := t.prefixCounts[prefix]
	n++
	t.prefixCounts[prefix] = n

	bv.name = prefix + "_" + strconv.Itoa(n)
	t.vars[key] = bv
	return bv
}

type BashCommand struct {
	parent   *BashTarget
	args     []string
	assignTo *BashVar
}

func (c *BashCommand) AssignTo(s fi.Unit) *BashCommand {
	bv := c.parent.vars[getKey(s)]
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

func (t *BashTarget) ReadVar(s fi.Unit) string {
	bv := t.vars[getKey(s)]
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

func (t *BashTarget) AddAWSTags(expected map[string]string, s fi.Unit, resourceType string) error {
	resourceId, exists := t.FindValue(s)
	var missing map[string]string
	if exists {
		actual, err := t.cloud.GetTags(resourceId, resourceType)
		if err != nil {
			return fmt.Errorf("unexpected error fetching tags for resource: %v", err)
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

func (t *BashTarget) AddAssignment(u fi.Unit, value string) {
	bv := t.vars[getKey(u)]
	if bv == nil {
		glog.Fatal("no variable assigned to ", u)
	}

	cmd := &BashCommand{}
	assign := bv.name + "=" + bashQuoteString(value)
	cmd.args = []string{assign}
	t.AddCommand(cmd)

	bv.staticValue = &value
}

func (t *BashTarget) FindValue(u fi.Unit) (string, bool) {
	bv := t.vars[getKey(u)]
	if bv == nil {
		glog.Fatal("no variable assigned to ", u)
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

func (t *BashTarget) AddLocalResource(r fi.Resource) (string, error) {
	switch r := r.(type) {
	case *fi.FileResource:
		return r.Path, nil
	}

	path := t.generateDynamicPath(fi.GetTypeName(r))
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

	err = fi.CopyResource(f, r)
	if err != nil {
		return "", fmt.Errorf("error writing resource: %v", err)
	}

	return path, nil
}

func (t *BashTarget) PutResource(key string, r fi.Resource) (string, string, error) {
	if r == nil {
		glog.Fatalf("Attempt to put null resource for %q", key)
	}
	return t.filestore.PutResource(key, r)
}

