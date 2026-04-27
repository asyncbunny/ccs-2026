RELAYER_TAG := $(shell grep '^ENV RELAYER_TAG' cosmos-relayer/Dockerfile | cut -f3 -d\ )
ANON_FULL_PATH := $(shell git rev-parse --show-toplevel)
ANON_VERSION_BEFORE_UPGRADE ?= v1.0.1

all: anond cosmos-relayer

anond: anond-rmi
	docker build --tag anon-org/anond -f anond/Dockerfile ${ANON_FULL_PATH}

anond-e2e:
	docker build --tag anon-org/anond -f anond/Dockerfile ${ANON_FULL_PATH} \
		--build-arg ANON_BUILD_OPTIONS="testnet e2e_upgrade"

anond-rmi:
	docker rmi anon-org/anond --force 2>/dev/null; true

e2e-init-chain-rmi:
	docker rmi anon-org/anond-e2e-init-chain --force 2>/dev/null; true

e2e-init-chain: e2e-init-chain-rmi
	@DOCKER_BUILDKIT=1 docker build -t anon-org/anond-e2e-init-chain --build-arg E2E_SCRIPT_NAME=chain --platform=linux/x86_64 \
		-f e2e-initialization/init.Dockerfile --build-arg VERSION="${ANON_VERSION_BEFORE_UPGRADE}" ${ANON_FULL_PATH}

cosmos-relayer: cosmos-relayer-rmi
	docker build --tag anon-org/cosmos-relayer:${RELAYER_TAG} -f cosmos-relayer/Dockerfile \
				${ANON_FULL_PATH}/contrib/images/cosmos-relayer
	docker tag anon-org/cosmos-relayer:${RELAYER_TAG} anon-org/cosmos-relayer:latest

cosmos-relayer-rmi:
	docker rmi anon-org/cosmos-relayer 2>/dev/null; true

.PHONY: all anond cosmos-relayer e2e-init-chain anond-rmi anond-rmi-upgrade cosmos-relayer-rmi
