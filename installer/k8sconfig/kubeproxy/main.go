package kubeproxy

import (
	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/files"
)

type KubeProxy struct {
	fi.StructuralUnit

	ApiServers            string
	LogLevel              string
	KubeproxyTestArgs     string
	KubeproxyTestLogLevel string
	KubeDockerRegistry    string
	KubeproxyDockerTag    string `id:"kube-proxy_docker_tag"`
}

func (k *KubeProxy) Add(c *fi.BuildContext) {
	c.Add(files.Path("/var/lib/kube-proxy/kubeconfig").WithMode(0400).DoTouch())
	c.Add(files.Path("/var/log/kube-proxy.log").DoTouch())

	c.Add(files.Path("/etc/kubernetes/manifests/kube-proxy.manifest").WithContents(k.buildManifest(c)))
}

func (k *KubeProxy) buildManifest(c *fi.BuildContext) fi.Resource {
	kubeconfig := "--kubeconfig=/var/lib/kube-proxy/kubeconfig"
	var api_servers string
	if k.ApiServers != "" {
		api_servers = "--master=https://" + k.ApiServers
	} else {
		panic("api_servers empty not implemented")
		//{% set ips = salt['mine.get']('roles:kubernetes-master', 'network.ip_addrs', 'grain').values() -%}
		//{% set api_servers = "--master=https://" + ips[0][0] -%}
	}

	cloud := c.Cloud()

	if cloud.IsGCE() || cloud.IsAWS() || cloud.IsVagrant() {
		// No change
	} else {
		api_servers += ":6443"
	}

	test_args := k.KubeproxyTestArgs

	log_level := k.LogLevel
	if k.KubeproxyTestLogLevel != "" {
		log_level = k.KubeproxyTestLogLevel
	}

	// TODO: Helper for adding args
	// kube-proxy {{api_servers_with_port}} {{kubeconfig}} --resource-container="" {{log_level}} {{test_args}} 1>>/var/log/kube-proxy.log 2>&1\n")
	exec := "kube-proxy "
	exec += api_servers + " "
	exec += kubeconfig + " "
	exec += `--resource-container="" `
	exec += log_level + " "
	exec += test_args + " "
	exec += "1>>/var/log/kube-proxy.log 2>&1"

	// TODO: Replace with k8s pod helper
	var sb fi.StringBuilder
	sb.Append("# kube-proxy podspec\n")
	sb.Append("apiVersion: v1\n")
	sb.Append("kind: Pod\n")
	sb.Append("metadata:\n")
	sb.Append("  name: kube-proxy\n")
	sb.Append("  namespace: kube-system\n")
	sb.Append("spec:\n")
	sb.Append("  hostNetwork: true\n")
	sb.Append("  containers:\n")
	sb.Append("  - name: kube-proxy\n")
	//sb.Append("    image: {{pillar['kube_docker_registry']}}/kube-proxy:{{pillar['kube-proxy_docker_tag']}}\n")
	sb.Append("    image: " + k.KubeDockerRegistry + "/kube-proxy:" + k.KubeproxyDockerTag + "\n")
	sb.Append("    command:\n")
	sb.Append("    - /bin/sh\n")
	sb.Append("    - -c\n")
	sb.Appendf("    - %s\n", exec)
	sb.Append("    securityContext:\n")
	sb.Append("      privileged: true\n")
	sb.Append("    volumeMounts:\n")
	sb.Append("    - mountPath: /etc/ssl/certs\n")
	sb.Append("      name: ssl-certs-host\n")
	sb.Append("      readOnly: true\n")
	sb.Append("    - mountPath: /var/log\n")
	sb.Append("      name: varlog\n")
	sb.Append("      readOnly: false\n")
	sb.Append("    - mountPath: /var/lib/kube-proxy/kubeconfig\n")
	sb.Append("      name: kubeconfig\n")
	sb.Append("      readOnly: false\n")
	sb.Append("  volumes:\n")
	sb.Append("  - hostPath:\n")
	sb.Append("      path: /usr/share/ca-certificates\n")
	sb.Append("    name: ssl-certs-host\n")
	sb.Append("  - hostPath:\n")
	sb.Append("      path: /var/lib/kube-proxy/kubeconfig\n")
	sb.Append("    name: kubeconfig\n")
	sb.Append("  - hostPath:\n")
	sb.Append("      path: /var/log\n")
	sb.Append("    name: varlog\n")
	sb.Append("}\n")

	return sb.AsResource()
}
