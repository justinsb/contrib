package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/tasks"
	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/fi/filestore"
	"k8s.io/contrib/installer/pkg/fi/ca"
)

type CreateClusterCmd struct {
	ClusterID string
	S3Bucket  string
	S3Region  string
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

	cmd.Flags().StringVar(&createCluster.S3Region, "s3-region", "", "Region in which to create the S3 bucket (if it does not exist)")
	cmd.Flags().StringVar(&createCluster.S3Bucket, "s3-bucket", "", "S3 bucket for upload of artifacts")

	cmd.Flags().StringVar(&createCluster.ClusterID, "cluster-id", "", "cluster id")
}

func (c*CreateClusterCmd) Run() error {
	k := &tasks.K8s{}
	k.Init()

	k.ClusterID = c.ClusterID

	// TODO: load config file

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
	castore, err := ca.NewCAStore("pki")
	if err != nil {
		return fmt.Errorf("error building CA store: %v", err)
	}


	//target := tasks.NewBashTarget(cloud, filestore)

	target := tasks.NewAWSAPITarget(cloud, filestore)

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

	return nil
}