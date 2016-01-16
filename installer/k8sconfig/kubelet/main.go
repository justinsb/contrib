package kubelet

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
	"github.com/kubernetes/contrib/installer/pkg/services"
)

type Args struct {
	args []string
}

func (a *Args) AddFlag(key, value string) {
	arg := "--" + key + "=" + value
	a.args = append(a.args, arg)
}

func (a *Args) AddFlagUnlessEmpty(key, value string) {
	if value == "" {
		return
	}
	a.AddFlag(key, value)
}

func (a *Args) Build() string {
	return strings.Join(a.args, " ")
}

type KubeletArgs struct {
	// Default: OneTwoThree => one-two-three
	// split words based on case; separate words with -; convert to lower
	ApiServers              string
	CloudProvider           string
	ReconcileCidr           bool `default:"true"`
	RegisterSchedulable     bool `default:"true"`
	EnableDebuggingHandlers bool
	Config                  string
	ManifestUrl             string
	ManifestUrlHeader       string
	ClusterDns              string
	ClusterDomain           string
	DockerRoot              string
	RootDir                 string
	HostnameOverride        string
	ConfigureCbr0           bool
	CgroupRoot              string
	SystemContainer         string
	PodCidr                 string
	CpuCfsQuota             bool
	Port                    int
}

func toArgName(s string) string {
	var b bytes.Buffer
	for i, r := range s {
		if unicode.IsUpper(r) {
			// New word
			if i != 0 {
				b.WriteRune('-')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}

	return string(b.Bytes())
}

func BuildArgs(cmd string, args interface{}) string {
	var argBuilder Args

	argsType := reflect.TypeOf(args)
	argsValue := reflect.ValueOf(args)

	for i := 0; i < argsValue.NumField(); i++ {
		field := argsType.Field(i)
		fieldName := field.Name
		fieldType := field.Type
		fieldValue := argsValue.Field(i)

		argValue := ""
		switch fieldType.Kind() {
		case reflect.Int:
			{
				v := fieldValue.Int()
				defaultValue := int64(0)
				defaultValueTag := field.Tag.Get("default")
				if defaultValueTag != "" {
					var err error
					defaultValue, err = strconv.ParseInt(defaultValueTag, 10, 64)
					if err != nil {
						panic("Unexpected error parsing default value: " + defaultValueTag + " for " + fieldName)
					}
				}
				if v != defaultValue {
					argValue = strconv.FormatInt(v, 10)
				}
			}

		default:
			panic(fmt.Sprintf("Unhandled field type: %v", fieldType.Kind()))
		}

		if argValue == "" {
			continue
		}

		argKey := toArgName(fieldName)
		argBuilder.AddFlag(argKey, argValue)
	}

	return cmd + " " + argBuilder.Build()
}

func buildKubeletCommandLine(c *fi.Context) string {
	var args KubeletArgs

	/* TODO:
	 {% set daemon_args = "$DAEMON_ARGS" -%}
		  {% if grains['os_family'] == 'RedHat' -%}
		    {% set daemon_args = "" -%}
		  {% endif -%}
	*/

	/*
	  {% if grains.api_servers is defined -%}
	    {% set api_servers = "--api-servers=https://" + grains.api_servers -%}
	  {% elif grains.apiservers is defined -%} # TODO(remove after 0.16.0): Deprecated form
	    {% set api_servers = "--api-servers=https://" + grains.apiservers -%}
	  {% elif grains['roles'][0] == 'kubernetes-master' -%}
	    {% set master_ipv4 = salt['grains.get']('fqdn_ip4')[0] -%}
	    {% set api_servers = "--api-servers=https://" + master_ipv4 -%}
	  {% else -%}
	    {% set ips = salt['mine.get']('roles:kubernetes-master', 'network.ip_addrs', 'grain').values() -%}
	    {% set api_servers = "--api-servers=https://" + ips[0][0] -%}
	  {% endif -%}
	*/
	if c.Get("api_servers") != "" {
		args.ApiServers = "https://" + c.Get("api_servers")
	} else {
		panic("api_servers not set")
	}

	cloud := c.Cloud()
	if cloud.IsAWS() || cloud.IsGCE() || cloud.IsVagrant() {
		// No port
	} else {
		panic("cloud not supported")

		/*
		  # TODO: remove nginx for other cloud providers.
		  {% if grains['cloud'] is defined and grains.cloud in [ 'aws', 'gce', 'vagrant' ]  %}
		    {% set api_servers_with_port = api_servers -%}
		  {% else -%}
		    {% set api_servers_with_port = api_servers + ":6443" -%}
		  {% endif -%}
		*/
	}

	args.EnableDebuggingHandlers = true

	if c.HasRole("kubernetes-master") {
		if cloud.IsAWS() || cloud.IsGCE() || cloud.IsVagrant() {
			// Unless given a specific directive, disable registration for the kubelet
			// running on the master.
			if c.Get("kubelet_api_servers") != "" {
				args.ApiServers = "https://" + c.Get("kubelet_api_servers")
				args.RegisterSchedulable = false
				args.ReconcileCidr = false
			} else {
				args.ApiServers = ""
			}

			// Disable the debugging handlers (/run and /exec) to prevent arbitrary
			// code execution on the master.
			// TODO(roberthbailey): Relax this constraint once the master is self-hosted.
			args.EnableDebuggingHandlers = false
		}
	}

	if cloud.ProviderID != "" && cloud.ProviderID != "vagrant" {
		args.CloudProvider = cloud.ProviderID
	}

	args.Config = "/etc/kubernetes/manifests"

	if c.GetBool("enable_manifest_url") {
		args.ManifestUrl = c.Get("manifest_url")
		args.ManifestUrlHeader = c.Get("manifest_url_header")
	}

	args.HostnameOverride = c.Get("hostname_override")

	if c.GetBool("enable_cluster_dns") {
		args.ClusterDns = c.Get("dns_server")
		args.ClusterDomain = c.Get("dns_domain")
	}

	args.DockerRoot = c.Get("docker_root")

	args.RootDir = c.Get("kubelet_root")

	args.ConfigureCbr0 = c.GetBool("allocate_node_cidrs")

	/*
	  # The master kubelet cannot wait for the flannel daemon because it is responsible
	  # for starting up the flannel server in a static pod. So even though the flannel
	  # daemon runs on the master, it doesn't hold up cluster bootstrap. All the pods
	  # on the master run with host networking, so the master flannel doesn't care
	  # even if the network changes. We only need it for the master proxy.
	  {% set experimental_flannel_overlay = "" -%}
	  {% if pillar.get('network_provider', '').lower() == 'flannel' and grains['roles'][0] != 'kubernetes-master' %}
	    {% set experimental_flannel_overlay = "--experimental-flannel-overlay=true" %}
	  {% endif -%}
	*/

	// Run containers under the root cgroup and create a system container.
	if c.OS().IsDebian() {
		args.SystemContainer = "/system"
		args.CgroupRoot = "/"
	}

	if c.OS().IsUbuntuVivid() {
		args.CgroupRoot = "docker"
	}

	if c.HasRole("kubernetes-master") {
		args.PodCidr = c.Get("cbd-cidr")
	}

	args.CpuCfsQuota = c.GetBool("enable_cpu_cfs_quota")

	if c.Get("kubelet_test_args") != "" {
		// args.AddArgs ???
		panic("kubelet_test_args not implemented")
		/*
		  {% set test_args = "" -%}
		  {% if pillar['kubelet_test_args'] is defined -%}
		    {% set test_args=pillar['kubelet_test_args'] %}
		  {% endif -%}
		*/
	}

	/*
	  {% set network_plugin = "" -%}
	  {% if pillar.get('network_provider', '').lower() == 'opencontrail' %}
	    {% set network_plugin = "--network-plugin=opencontrail" %}
	  {% endif -%}
	*/

	args.Port = c.GetInt("kubelet_port")

	if c.Get("kubelet_test_log_level") != "" {
		panic("kubelet_test_log_level not implemented")
		/*
		  {% set log_level = pillar['log_level'] -%}
		  {% if pillar['kubelet_test_log_level'] is defined -%}
		    {% set log_level = pillar['kubelet_test_log_level'] -%}
		  {% endif -%}
		*/
	}

	// test_args has to be kept at the end, so they'll overwrite any prior configuration
	/*DAEMON_ARGS="{{daemon_args}} {{api_servers_with_port}} {{debugging_handlers}} {{hostname_override}} {{cloud_provider}} {{config}} {{manifest_url}} --allow-privileged={{pillar['allow_privileged']}} {{log_level}} {{cluster_dns}} {{cluster_domain}} {{docker_root}} {{kubelet_root}} {{configure_cbr0}} {{cgroup_root}} {{system_container}} {{pod_cidr}} {{ master_kubelet_args }} {{cpu_cfs_quota}} {{network_plugin}} {{kubelet_port}} {{experimental_flannel_overlay}} {{test_args}}"
	 */

	return BuildArgs("/usr/local/bin/kubelet", args)
}

func Add(context *fi.Context) {
	context.Add(files.Path("/usr/local/bin/kubelet").WithMode(0755).WithContents(fi.Resource("kubelet")))

	// The default here is that this file is blank. If this is the case, the kubelet
	// won't be able to parse it as JSON and it will not be able to publish events
	// to the apiserver. You'll see a single error line in the kubelet start up file
	// about this.
	context.Add(files.Path("/var/lib/kubelet/kubeconfig").WithMode(0400).WithContents(fi.StaticContent("")))

	kubeletExec := buildKubeletCommandLine(context)

	s := services.Running("kubelet")
	s.Description = "Kubernetes Kubelet Server"
	s.Documentation = "https://github.com/GoogleCloudPlatform/kubernetes"
	s.Exec = kubeletExec
	context.Add(s)

	/*{% if pillar.get('is_systemd') %}
	  {% set environment_file = '/etc/sysconfig/kubelet' %}
	  {% else %}
	  {% set environment_file = '/etc/default/kubelet' %}
	  {% endif %}

	  {{ environment_file}}:
	    file.managed:
	      - source: salt://kubelet/default
	      - template: jinja
	      - user: root
	      - group: root
	      - mode: 644

	  /usr/local/bin/kubelet:
	    file.managed:
	      - source: salt://kube-bins/kubelet
	      - user: root
	      - group: root
	      - mode: 755

	  # The default here is that this file is blank. If this is the case, the kubelet
	  # won't be able to parse it as JSON and it will not be able to publish events
	  # to the apiserver. You'll see a single error line in the kubelet start up file
	  # about this.
	  /var/lib/kubelet/kubeconfig:
	    file.managed:
	      - source: salt://kubelet/kubeconfig
	      - user: root
	      - group: root
	      - mode: 400
	      - makedirs: true


	  {% if pillar.get('is_systemd') %}

	  {{ pillar.get('systemd_system_path') }}/kubelet.service:
	    file.managed:
	      - source: salt://kubelet/kubelet.service
	      - user: root
	      - group: root

	  # The service.running block below doesn't work reliably
	  # Instead we run our script which e.g. does a systemd daemon-reload
	  # But we keep the service block below, so it can be used by dependencies
	  # TODO: Fix this
	  fix-service-kubelet:
	    cmd.wait:
	      - name: /opt/kubernetes/helpers/services bounce kubelet
	      - watch:
	        - file: /usr/local/bin/kubelet
	        - file: {{ pillar.get('systemd_system_path') }}/kubelet.service
	        - file: {{ environment_file }}
	        - file: /var/lib/kubelet/kubeconfig

	  {% else %}

	  /etc/init.d/kubelet:
	    file.managed:
	      - source: salt://kubelet/initd
	      - user: root
	      - group: root
	      - mode: 755

	  {% endif %}

	  kubelet:
	    service.running:
	      - enable: True
	      - watch:
	        - file: /usr/local/bin/kubelet
	  {% if pillar.get('is_systemd') %}
	        - file: {{ pillar.get('systemd_system_path') }}/kubelet.service
	  {% else %}
	        - file: /etc/init.d/kubelet
	  {% endif %}
	  {% if grains['os_family'] == 'RedHat' %}
	        - file: /usr/lib/systemd/system/kubelet.service
	  {% endif %}
	        - file: {{ environment_file }}
	        - file: /var/lib/kubelet/kubeconfig
	*/
}
