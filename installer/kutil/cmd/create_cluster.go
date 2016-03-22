package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/tasks"
	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/fi/filestore"
	"k8s.io/contrib/installer/pkg/fi/ca"
	"path"
	"os"
	"io/ioutil"
	"golang.org/x/crypto/ssh"
)

type CreateClusterCmd struct {
	ClusterID  string
	S3Bucket   string
	S3Region   string
	SSHKey     string
	StateDir   string
	ReleaseDir string
	Target     string
}

var createCluster CreateClusterCmd

func init() {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Create cluster",
		Long: `Creates a new k8s cluster.`,
		Run: func(cmd *cobra.Command, args[]string) {
			err := createCluster.Run()
			if err != nil {
				glog.Exitf("%v", err)
			}
		},
	}

	createCmd.AddCommand(cmd)

	cmd.Flags().StringVarP(&createCluster.StateDir, "dir", "d", "", "Directory to load & store state")
	cmd.Flags().StringVarP(&createCluster.ReleaseDir, "release", "r", "", "Directory to load release from")
	cmd.Flags().StringVar(&createCluster.S3Region, "s3-region", "", "Region in which to create the S3 bucket (if it does not exist)")
	cmd.Flags().StringVar(&createCluster.S3Bucket, "s3-bucket", "", "S3 bucket for upload of artifacts")
	cmd.Flags().StringVarP(&createCluster.SSHKey, "i", "i", "", "SSH Key for cluster")
	cmd.Flags().StringVarP(&createCluster.Target, "target", "t", "direct", "Target type.  Suported: direct, bash")

	cmd.Flags().StringVar(&createCluster.ClusterID, "cluster-id", "", "cluster id")
}

func (c*CreateClusterCmd) Run() error {
	k := &tasks.K8s{}
	k.Init()

	k.ClusterID = c.ClusterID

	if c.SSHKey != "" {
		buffer, err := ioutil.ReadFile(c.SSHKey)
		if err != nil {
			return fmt.Errorf("error reading SSH key file %q: %v", c.SSHKey, err)
		}

		privateKey, err := ssh.ParsePrivateKey(buffer)
		if err != nil {
			return fmt.Errorf("error parsing key file %q: %v", c.SSHKey, err)
		}

		publicKey := privateKey.PublicKey()
		authorized := ssh.MarshalAuthorizedKey(publicKey)

		k.SSHPublicKey = fi.NewStringResource(string(authorized))
	}

	if c.StateDir == "" {
		return fmt.Errorf("state dir is required")
	}

	if c.ReleaseDir == "" {
		return fmt.Errorf("release dir is required")
	}

	{
		confFile := path.Join(c.StateDir, "kubernetes.yaml")
		b, err := ioutil.ReadFile(confFile)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("error loading state file %q: %v", confFile, err)
			}
		}
		glog.Infof("Loading state from %q", confFile)
		err = k.MergeState(b)
		if err != nil {
			return fmt.Errorf("error parsing state file %q: %v", confFile, err)
		}
	}

	if k.SSHPublicKey == nil {
		// TODO: Implement the generation logic
		return fmt.Errorf("ssh key is required (for now!).  Specify with -i")
	}

	k.ServerBinaryTar = fi.NewFileResource(path.Join(c.ReleaseDir, "server/kubernetes-server-linux-amd64.tar.gz"))
	k.SaltTar = fi.NewFileResource(path.Join(c.ReleaseDir, "server/kubernetes-salt.tar.gz"))

	glog.V(4).Infof("Configuration is %s", tasks.DebugPrint(k))

	if k.ClusterID == "" {
		return fmt.Errorf("ClusterID is required")
	}

	az := k.Zone
	if len(az) <= 2 {
		return fmt.Errorf("Invalid AZ: ", az)
	}
	region := az[:len(az) - 1]
	if c.S3Region == "" {
		c.S3Region = region
	}

	if c.S3Bucket == "" {
		// TODO: Implement the generation logic
		return fmt.Errorf("s3-bucket is required (for now!)")
	}

	tags := map[string]string{"KubernetesCluster": k.ClusterID}
	cloud := fi.NewAWSCloud(region, tags)

	s3Bucket, err := cloud.S3.EnsureBucket(c.S3Bucket, c.S3Region)
	if err != nil {
		return fmt.Errorf("error creating s3 bucket: %v", err)
	}
	s3Prefix := "devel/" + k.ClusterID + "/"
	filestore := filestore.NewS3FileStore(s3Bucket, s3Prefix)
	castore, err := ca.NewCAStore(path.Join(c.StateDir, "pki"))
	if err != nil {
		return fmt.Errorf("error building CA store: %v", err)
	}

	var target fi.Target
	var bashTarget *tasks.BashTarget

	switch (c.Target) {
	case "direct":
		target = tasks.NewAWSAPITarget(cloud, filestore)
	case "bash":
		bashTarget = tasks.NewBashTarget(cloud, filestore)
		target = bashTarget
	default:
		return fmt.Errorf("unsupported target type %q", c.Target)
	}

	// TODO: Rationalize configs
	config := fi.NewSimpleConfig()
	//if configPath != "" {
	//	panic("additional config not supported yet")
	//	//err := fi.Config.ReadYaml(configPath)
	//	//if err != nil {
	//	//	glog.Fatalf("error reading configuration: %v", err)
	//	//}
	//}

	context, err := fi.NewContext(config, cloud, castore)
	if err != nil {
		return fmt.Errorf("error building config: %v", err)
	}

	bc := context.NewBuildContext()
	bc.Add(k)

	runMode := fi.ModeConfigure
	//if validate {
	//	runMode = fi.ModeValidate
	//}

	rc := context.NewRunContext(target, runMode)
	err = rc.Run()
	if err != nil {
		return fmt.Errorf("error running configuration: %v", err)
	}

	if bashTarget != nil {
		err = bashTarget.PrintShellCommands(os.Stdout)
		if err != nil {
			glog.Fatal("error building shell commands: %v", err)
		}
	}

	return nil
}
