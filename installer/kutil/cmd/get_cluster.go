package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/kutil/pkg/kutil"
)

type GetClusterCmd struct {
	Region    string
	Zone      string
	ClusterID string
}

var getCluster GetClusterCmd

func init() {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "List cluster",
		Long: `Lists k8s cluster.`,
		Run: func(cmd *cobra.Command, args[]string) {
			if len(args) != 0 {
				if len(args) == 1 {
					getCluster.ClusterID = args[0]
				} else {
					glog.Exitf("unexpected arguments passed")
				}
			}
			err := getCluster.Run()
			if err != nil {
				glog.Exitf("%v", err)
			}
		},
	}

	getCmd.AddCommand(cmd)

	cmd.Flags().StringVar(&getCluster.Region, "region", "", "region")
	cmd.Flags().StringVar(&getCluster.Zone, "zone", "", "zone")
}

func (c*GetClusterCmd) Run() error {
	if c.Zone == "" && c.Region == "" {
		return fmt.Errorf("--zone or --region is required")
	}

	if c.Region == "" {
		az := c.Zone
		if len(az) <= 2 {
			return fmt.Errorf("Invalid AZ: ", az)
		}
		c.Region = az[:len(az) - 1]
	}

	tags := make(map[string]string)
	cloud := fi.NewAWSCloud(c.Region, tags)

	var clusterIDs []string

	if c.ClusterID == "" {
		d := &kutil.ListClusters{}

		d.Region = c.Region
		d.Cloud = cloud

		clusters, err := d.ListClusters()
		if err != nil {
			return err
		}

		for _, c := range clusters {
			clusterIDs = append(clusterIDs, c.ClusterID)
		}
	} else {
		clusterIDs = append(clusterIDs, c.ClusterID)
	}

	for _, c := range clusterIDs {
		gi := &kutil.GetClusterInfo{Cloud: cloud, ClusterID: c}
		info, err := gi.GetClusterInfo()
		if err != nil {
			return err
		}
		if info == nil {
			fmt.Printf("%v\t%v\t%v\n", c, "?", "?")
			continue
		}
		fmt.Printf("%v\t%v\t%v\n", info.ClusterID, info.MasterIP, info.Zone)
	}
	return nil
}