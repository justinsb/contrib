code:
	go generate ./...
	go install ./...

push: code
	rsync -avz ${GOPATH}/bin/bootstrap ubuntu@${REMOTE}:/tmp/

run: push
	ssh ubuntu@${REMOTE} "chmod +x /tmp/bootstrap && sudo time /tmp/bootstrap --v=2 --resources=/var/cache/salt/minion/files/base/kube-bins/ --roles=kubernetes-pool"

