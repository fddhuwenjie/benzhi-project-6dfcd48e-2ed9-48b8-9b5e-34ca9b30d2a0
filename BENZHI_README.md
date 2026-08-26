# BENZHI_README

## 项目说明
- 项目：benzhi-project-6dfcd48e-2ed9-48b8-9b5e-34ca9b30d2a0
- 项目用途：已完整实现面向声像档案保存机构的磁带载体退化事件处置 HTTP API，覆盖事件登记、抽样圈定、双人方案审批、逐盘处理、量化复验、独立裁定、确定性封存、审计时间线与摘要校验。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 项目描述
- 项目名称：tape-preservation-incident-api
- 项目介绍：面向声像档案保存机构的磁带载体退化事件处置服务，将异常批次从登记、影响圈定、隔离评估、稳定化处理、可读性复验推进到独立放行与证据封存，确保每次状态变化都有可验证依据。项目根目录必须提供简体中文 README.md，说明用途以及标准构建、运行和测试方式。
- 项目概述：面向声像档案保存机构的磁带载体退化事件处置服务，将异常批次从登记、影响圈定、隔离评估、稳定化处理、可读性复验推进到独立放行与证据封存，确保每次状态变化都有可验证依据。项目根目录必须提供简体中文 README.md，说明用途以及标准构建、运行和测试方式。
- 核心工作流：保存工程师登记磁带载体退化事件并冻结涉事批次，完成抽样检查与影响圈定后提交稳定化方案；执行处理并记录逐盘结果，对处理后的磁带开展可读性复验，最后由未参与处置的复核员签发放行或报废裁定并封存完整证据。
- 对外接口：提供版本化 HTTP JSON API，覆盖事件建档、抽样与圈定、稳定化方案审批、处理记录、可读性复验、独立裁定及档案校验；服务监听地址支持 -addr=127.0.0.1:<port>，默认使用 127.0.0.1:19091，不默认绑定 8080、80、3000 或 0.0.0.0。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/server -self-check -addr=127.0.0.1:19091

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-6dfcd48e-2ed9-48b8-9b5e-34ca9b30d2a0-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-6dfcd48e-2ed9-48b8-9b5e-34ca9b30d2a0-arm64 linux/arm64

docker run -it benzhi-project-6dfcd48e-2ed9-48b8-9b5e-34ca9b30d2a0-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -self-check -addr=127.0.0.1:19091`
