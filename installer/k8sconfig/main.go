package k8sconfig

import (
	"github.com/kubernetes/contrib/installer/k8sconfig/debianautoupgrades"
	"github.com/kubernetes/contrib/installer/k8sconfig/docker"
	"github.com/kubernetes/contrib/installer/k8sconfig/kubeclienttools"
	"github.com/kubernetes/contrib/installer/k8sconfig/kubelet"
	"github.com/kubernetes/contrib/installer/k8sconfig/kubenodeunpacker"
	"github.com/kubernetes/contrib/installer/k8sconfig/kubeproxy"
	"github.com/kubernetes/contrib/installer/k8sconfig/logrotate"
	"github.com/kubernetes/contrib/installer/k8sconfig/ntp"
	"github.com/kubernetes/contrib/installer/pkg/fi"
)

func Add(c *fi.Context) {
	addBase(c)
	debianautoupgrades.Add(c)
	//salthelpers.Add(c)
	if c.Cloud().IsAWS() {
		ntp.Add(c)
	}
	/*{% if pillar.get('e2e_storage_test_environment', '').lower() == 'true' %}
	  - e2e
	  {% endif %}
	*/

	if c.HasRole("kubernetes-pool") {
		// TODO: match: grain
		docker.Add(c)
		/*{% if pillar.get('network_provider', '').lower() == 'flannel' %}
		  - flannel
		  {% endif %}
		*/
		// safe_format_and_mount is now unused (?) helpers.Add(c)
		// cadvisor is now part of kubelet cadvisor.Add(c)
		kubeclienttools.Add(c)
		kubenodeunpacker.Add(c)
		kubelet.Add(c)
		/*{% if pillar.get('network_provider', '').lower() == 'opencontrail' %}
		    - opencontrail-networking-minion
		    {% else %}
		 - kube-proxy
			{% endif %}
		*/
		kubeproxy.Add(c)

		/*
			{% if pillar.get('enable_node_logging', '').lower() == 'true' and pillar['logging_destination'] is defined %}
			  {% if pillar['logging_destination'] == 'elasticsearch' %}
			      - fluentd-es
			        {% elif pillar['logging_destination'] == 'gcp' %}
				    - fluentd-gcp
				      {% endif %}
				      {% endif %}
		*/

		/*
			      {% if pillar.get('enable_cluster_registry', '').lower() == 'true' %}
			          - kube-registry-proxy
				  {% endif %}
		*/

		logrotate.Add(c)
		// Not for systemd
		//supervisor.Add(c)
	}

	if c.HasRole("kubernetes-master") {
		// TODO: - match: grain
		panic("Not implemented")
		/*
			    - generate-cert
			        - etcd
				{% if pillar.get('network_provider', '').lower() == 'flannel' %}
				    - flannel-server
				        - flannel
					{% endif %}
					    - kube-apiserver
					        - kube-controller-manager
						    - kube-scheduler
						        - supervisor
							{% if grains['cloud'] is defined and not grains.cloud in [ 'aws', 'gce', 'vagrant' ] %}
							    - nginx
							    {% endif %}
							        - cadvisor
								    - kube-client-tools
								        - kube-master-addons
									    - kube-admission-controls
									    {% if pillar.get('enable_node_logging', '').lower() == 'true' and pillar['logging_destination'] is defined %}
									      {% if pillar['logging_destination'] == 'elasticsearch' %}
									          - fluentd-es
										    {% elif pillar['logging_destination'] == 'gcp' %}
										        - fluentd-gcp
											  {% endif %}
											  {% endif %}
											  {% if grains['cloud'] is defined and grains['cloud'] != 'vagrant' %}
											      - logrotate
											      {% endif %}
											          - kube-addons
												  {% if grains['cloud'] is defined and grains['cloud'] in [ 'vagrant', 'gce', 'aws' ] %}
												      - docker
												          - kubelet
													  {% endif %}
													  {% if pillar.get('network_provider', '').lower() == 'opencontrail' %}
													      - opencontrail-networking-master
													      {% endif %}
		*/

	}
}
