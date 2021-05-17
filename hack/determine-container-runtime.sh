# problem 1: my user is not part of the `docker` group, thus i need `sudo kind`
# problem 2: if using podman, `sudo kind` is also needed because the kubelet cannot run in an unprivileged container
# solution: test for these conditions and set
# - $CONTAINER_COMMAND
# - $KIND_COMMAND
# - $KIND_EXPERIMENTAL_PROVIDER
# accordingly
# to override: set CONTAINER_RUNTIME to either docker or podman

function __export_docker() {
	# prepend sudo if user is not in docker group
	if [[ -z "$(groups | grep docker)" ]]; then
		export CONTAINER_COMMAND="sudo docker"
		export KIND_COMMAND="sudo kind"
	else
		export CONTAINER_COMMAND="docker"
		export KIND_COMMAND="kind"
	fi
}

function __export_podman() {
	export KIND_EXPERIMENTAL_PROVIDER=podman
	export CONTAINER_COMMAND=podman
	export KIND_COMMAND="sudo --preserve-env=KIND_EXPERIMENTAL_PROVIDER kind"
}

##############################
# SCRIPT EXECUTION STARTS HERE
##############################

if [[ -n "${CONTAINER_RUNTIME:-}" ]]; then
	echo "CONTAINER_RUNTIME is set to: $CONTAINER_RUNTIME"
	echo "forcing container runtime"
else
	echo "CONTAINER_RUNTIME is not set or empty"
	echo "detecting container runtime"

	if [[ -f "$(command -v podman)" ]]; then
		CONTAINER_RUNTIME="podman"
	elif [[ -f "$(command -v docker)" ]]; then
		CONTAINER_RUNTIME="docker"
	fi
fi

case "$CONTAINER_RUNTIME" in
	docker)
		__export_docker
	;;
	podman)
		__export_podman
	;;
	*)
		echo "Unknown container runtime: $CONTAINER_RUNTIME"
		echo "Please use either docker or podman"
esac

echo "detected container runtime: $CONTAINER_RUNTIME"
echo
