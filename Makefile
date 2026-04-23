#
# 如果版本发生变化需要修改这里的版本号，或者在命令行指定版本号，如: make VER=1.0.0110 deploy
# build.py中的版本号可以通过参数指定，无需改动
# 如果要在生产环境部署应用可以指定ENV参数，如: make ENV=prod deploy
#
APP    := efficiency-dashboard-backend
VER    := 1.0.2
OS     := $(shell go env GOOS)
ARCH   := $(shell go env GOARCH)
ENV    := prod

SHENMA_DOCKER_REPO  := $(shell grep '^SHENMA_DOCKER_REPO=' ./.env | cut -d '=' -f 2-)
SHENMA_DOCKER_HOST  := $(shell grep '^SHENMA_DOCKER_HOST=' ./.env | cut -d '=' -f 2-)
SHENMA_DOCKER_USER  := $(shell grep '^SHENMA_DOCKER_USER=' ./.env | cut -d '=' -f 2-)
SHENMA_DOCKER_PASSWORD := $(shell grep '^SHENMA_DOCKER_PASSWORD=' ./.env | cut -d '=' -f 2-)

#ENV := prod
EXEEXT ?= 
ifeq (windows,$(OS))
EXEEXT := .exe
endif

ifdef DEBUG
DEBUGOPT := '--debug'
else
DEBUGOPT := 
endif
# 构建
build:
	cd backend && python ../build.py --software $(VER) $(DEBUGOPT)

docs:
	cd backend && swag init

# 打镜像包
package:
	docker build --build-arg VERSION=$(VER) -f Dockerfile . -t zgsm/$(APP):$(VER)

# 上传镜像包到dockerhub
upload_dockerhub:
	docker tag $(APP):$(VER) $(SHENMA_DOCKER_REPO)/$(APP):$(VER)
	docker login $(SHENMA_DOCKER_HOST) -u $(SHENMA_DOCKER_USER) -p $(SHENMA_DOCKER_PASSWORD)
	docker push $(SHENMA_DOCKER_REPO)/$(APP):$(VER)

# 上传镜像包到制品库和前置harbor
upload: upload_dockerhub

.PHONY: docs build package upload deploy upload_dockerhub clean 
