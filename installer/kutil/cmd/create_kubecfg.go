package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/golang/glog"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"k8s.io/contrib/installer/kutil/pkg/kutil"
	"strings"
	"os"
	"encoding/base64"
	"path"
)

type CreateKubecfgCmd struct {
	Master         string
	SSHIdentity    string
	UseKubeletCert bool
}

var createKubecfg CreateKubecfgCmd

func init() {
	cmd := &cobra.Command{
		Use:   "kubecfg",
		Short: "Create kubecfg file from master",
		Long: `Connects to your master server over SSH, and builds a kubecfg file from the settings.`,
		Run: func(cmd *cobra.Command, args[]string) {
			err := createKubecfg.Run()
			if err != nil {
				glog.Exitf("%v", err)
			}
		},
	}

	createCmd.AddCommand(cmd)

	cmd.Flags().StringVarP(&createKubecfg.Master, "master", "m", "", "Master IP address or hostname")
	cmd.Flags().StringVarP(&createKubecfg.SSHIdentity, "i", "i", "", "SSH private key")
	cmd.Flags().BoolVar(&createKubecfg.UseKubeletCert, "use-kubelet-cert", false, "Build using the kublet cert (useful if the kubecfg cert is not available)")
}

func parsePrivateKeyFile(p string) (ssh.AuthMethod, error) {
	buffer, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("error reading key file %q: %v", p, err)
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, fmt.Errorf("error parsing key file %q: %v", p, err)
	}
	return ssh.PublicKeys(key), nil
}

func (c*CreateKubecfgCmd) Run() error {
	if c.Master == "" {
		return fmt.Errorf("--master must be specified")
	}
	fmt.Printf("Connecting to %s\n", c.Master)

	sshConfig := &ssh.ClientConfig{
		User: "admin",
	}

	if c.SSHIdentity != "" {
		a, err := parsePrivateKeyFile(c.SSHIdentity)
		if err != nil {
			return err
		}
		sshConfig.Auth = append(sshConfig.Auth, a)
	}

	sshClient, err := ssh.Dial("tcp", c.Master + ":22", sshConfig)
	if err != nil {
		return fmt.Errorf("error connecting to SSH on server %q: %v", c.Master, err)
	}

	sshSession, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("error creating SSH session: %v", err)
	}

	//output, err := sshSession.CombinedOutput("cat /etc/kubernetes/kube_env.yaml")
	//if err != nil {
	//	return fmt.Errorf("error running SSH command: %v", err)
	//}

	output, err := sshSession.CombinedOutput("curl -s http://169.254.169.254/latest/user-data")
	if err != nil {
		return fmt.Errorf("error running SSH command: %v", err)
	}

	conf := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		sep := strings.Index(line, ": ")
		k := ""
		v := ""
		if sep != -1 {
			k = line[0:sep]
			v = line[sep + 2:]
		}

		if k == "" {
			glog.V(4).Infof("Unknown line: %s", line)
		}

		if len(v) >= 2 && v[0] == '"' && v[len(v) - 1] == '"' {
			v = v[1:len(v) - 1]
		}
		conf[k] = v
	}

	instancePrefix := conf["INSTANCE_PREFIX"]
	if instancePrefix == "" {
		return fmt.Errorf("cannot determine INSTANCE_PREFIX")
	}

	tmpdir, err := ioutil.TempDir("", "k8s")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	b := &kutil.KubeconfigBuilder{}
	b.Init()
	b.Context = "aws_" + instancePrefix

	//glog.Infof("Output: %v", string(output))

	caCertPath, err := confToFile(tmpdir, conf, "CA_CERT")
	if err != nil {
		return err
	}

	kubecfgCertConfKey := "KUBECFG_CERT"
	kubecfgKeyConfKey := "KUBECFG_KEY"
	if c.UseKubeletCert {
		kubecfgCertConfKey = "KUBELET_CERT"
		kubecfgKeyConfKey = "KUBELET_KEY"
	} else {
		if conf[kubecfgCertConfKey] == "" {
			fmt.Printf("%s was not found in the configuration; you may want to specify --use-kubelet-cert\n", kubecfgCertConfKey)
		}
	}

	kubeCertPath, err := confToFile(tmpdir, conf, kubecfgCertConfKey)
	if err != nil {
		return err
	}
	kubeKeyPath, err := confToFile(tmpdir, conf, kubecfgKeyConfKey)
	if err != nil {
		return err
	}

	b.CACert = caCertPath
	b.KubeCert = kubeCertPath
	b.KubeKey = kubeKeyPath
	b.KubeMasterIP = c.Master

	err = b.CreateKubeconfig()
	if err != nil {
		return err
	}

	return nil
}

func confToFile(tmpdir string, conf map[string]string, key string) (string, error) {
	v, found := conf[key]
	if !found {
		return "", fmt.Errorf("cannot find configuration value for %q", key)
	}

	if v == "" {
		return "", fmt.Errorf("configuration value for %q was unexpectedly empty", key)
	}

	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return "", fmt.Errorf("configuration value was not base64-encoded for %s", key)
	}

	p := path.Join(tmpdir, key)
	err = ioutil.WriteFile(p, b, 0700)
	if err != nil {
		return "", fmt.Errorf("error writing configuration value to file %q: %v", p, err)
	}

	return p, nil
}