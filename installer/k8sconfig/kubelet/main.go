package kubelet

import (
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
	"github.com/kubernetes/contrib/installer/pkg/services"
)

func buildKubeletCommandLine(c *fi.Context) string {
	cmd := "/usr/local/bin/kubelet "

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
	api_servers = ""
	if c.Get("api_servers") != "" {
		api_servers = " --api-servers=https://" + c.get("api_servers")
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

	master_kubelet_args := ""
	debugging_handlers := " --enable-debugging-handlers=true"

	if c.HasRole("kubernetes-master") {
		if cloud.IsAWS() || cloud.IsGCE() || cloud.IsVagrant() {
			// Unless given a specific directive, disable registration for the kubelet
			// running on the master.
			if c.Get("kubelet_api_servers") != "" {
				api_servers = " --api-servers=https://" + c.Get("kubelet_api_servers")
				master_kubelet_args += " --register-schedulable=false --reconcile-cidr=false"
			} else {
				api_servers = ""
			}

			// Disable the debugging handlers (/run and /exec) to prevent arbitrary
			// code execution on the master.
			// TODO(roberthbailey): Relax this constraint once the master is self-hosted.
			debugging_handlers = " --enable-debugging-handlers=false"
		}
	}

	cloud_provider := ""
	if cloud.GetProviderId() != "" && cloud.GetProviderId() != "vagrant" {
		cloud_provider = " --cloud-provider=" + cloud.GetProviderId()
	}

	config := " --config=/etc/kubernetes/manifests"

	manifest_url := ""
	manifest_url_header := ""

	if c.GetBool("enable_manifest_url") {
		manifest_url = " --manifest-url=" + c.Get("manifest_url") + " --manifest-url-header=" + c.Get("manifest_url_header")
	}

	hostname_override := ""
	if c.Get("hostname_override") != "" {
		hostname_override = " --hostname-override=" + c.Get("hostname_override")
	}

	cluster_dns := ""
	cluster_domain := ""
	if c.GetBool("enable_cluster_dns") {
		cluster_dns = " --cluster-dns=" + c.Get("dns_server")
		cluster_domain = " --cluster-domain=" + c.Get("dns_domain")
	}

	docker_root := ""
	if c.Get("docker_root") {
		docker_root = " --docker-root=" + c.Get("docker_root")
	}

	kubelet_root := ""
	if c.Get("kubelet_root") != "" {
		kubelet_root = " --root-dir=" + c.Get("kubelet_root")
	}

	/*
	  {% set configure_cbr0 = "" -%}
	  {% if pillar['allocate_node_cidrs'] is defined -%}
	    {% set configure_cbr0 = "--configure-cbr0=" + pillar['allocate_node_cidrs'] -%}
	  {% endif -%}

	  # The master kubelet cannot wait for the flannel daemon because it is responsible
	  # for starting up the flannel server in a static pod. So even though the flannel
	  # daemon runs on the master, it doesn't hold up cluster bootstrap. All the pods
	  # on the master run with host networking, so the master flannel doesn't care
	  # even if the network changes. We only need it for the master proxy.
	  {% set experimental_flannel_overlay = "" -%}
	  {% if pillar.get('network_provider', '').lower() == 'flannel' and grains['roles'][0] != 'kubernetes-master' %}
	    {% set experimental_flannel_overlay = "--experimental-flannel-overlay=true" %}
	  {% endif -%}

	  # Run containers under the root cgroup and create a system container.
	  {% set system_container = "" -%}
	  {% set cgroup_root = "" -%}
	  {% if grains['os_family'] == 'Debian' -%}
	    {% set system_container = "--system-container=/system" -%}
	    {% set cgroup_root = "--cgroup-root=/" -%}
	  {% endif -%}
	  {% if grains['oscodename'] == 'vivid' -%}
	    {% set cgroup_root = "--cgroup-root=docker" -%}
	  {% endif -%}

	  {% set pod_cidr = "" %}
	  {% if grains['roles'][0] == 'kubernetes-master' and grains.get('cbr-cidr') %}
	    {% set pod_cidr = "--pod-cidr=" + grains['cbr-cidr'] %}
	  {% endif %}

	  {% set cpu_cfs_quota = "" %}
	  {% if pillar['enable_cpu_cfs_quota'] is defined -%}
	   {% set cpu_cfs_quota = "--cpu-cfs-quota=" + pillar['enable_cpu_cfs_quota'] -%}
	  {% endif -%}

	  {% set test_args = "" -%}
	  {% if pillar['kubelet_test_args'] is defined -%}
	    {% set test_args=pillar['kubelet_test_args'] %}
	  {% endif -%}

	  {% set network_plugin = "" -%}
	  {% if pillar.get('network_provider', '').lower() == 'opencontrail' %}
	    {% set network_plugin = "--network-plugin=opencontrail" %}
	  {% endif -%}

	  {% set kubelet_port = "" -%}
	  {% if pillar['kubelet_port'] is defined -%}
	    {% set kubelet_port="--port=" + pillar['kubelet_port'] %}
	  {% endif -%}

	  {% set log_level = pillar['log_level'] -%}
	  {% if pillar['kubelet_test_log_level'] is defined -%}
	    {% set log_level = pillar['kubelet_test_log_level'] -%}
	  {% endif -%}

	  # test_args has to be kept at the end, so they'll overwrite any prior configuration
	  DAEMON_ARGS="{{daemon_args}} {{api_servers_with_port}} {{debugging_handlers}} {{hostname_override}} {{cloud_provider}} {{config}} {{manifest_url}} --allow-privileged={{pillar['allow_privileged']}} {{log_level}} {{cluster_dns}} {{cluster_domain}} {{docker_root}} {{kubelet_root}} {{configure_cbr0}} {{cgroup_root}} {{system_container}} {{pod_cidr}} {{ master_kubelet_args }} {{cpu_cfs_quota}} {{network_plugin}} {{kubelet_port}} {{experimental_flannel_overlay}} {{test_args}}"
	*/
}

func Add(context *fi.Context) {
	context.Add(files.Path("/usr/local/bin/kubelet").WithMode(0755).WithContents(fi.Resource("kubelet")))

	// The default here is that this file is blank. If this is the case, the kubelet
	// won't be able to parse it as JSON and it will not be able to publish events
	// to the apiserver. You'll see a single error line in the kubelet start up file
	// about this.
	context.Add(files.Path("/var/lib/kubelet/kubeconfig").WithMode(0400).WithContents(fi.StaticContents("")))

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
