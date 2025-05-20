# Kubernetes Pod Lifecycle Monitor

这是一个使用 Kubernetes Informer 来监控 Pod 生命周期事件的简单程序。它可以监听集群中所有 Pod 的创建、更新和删除事件，并打印出相关的状态信息。

## 功能特点

- 监听所有 Pod 的创建事件
- 监听所有 Pod 的更新事件
- 监听所有 Pod 的删除事件
- 打印 Pod 的命名空间、名称和状态信息

## 前置条件

- Go 1.21 或更高版本
- 有效的 Kubernetes 配置（kubeconfig）

## 安装

1. 克隆仓库：
```bash
git clone <repository-url>
cd billing-job
```

2. 安装依赖：
```bash
go mod tidy
```

## 运行

确保你有正确的 Kubernetes 配置（kubeconfig），然后运行：

```bash
go run main.go
```

程序会自动连接到 Kubernetes 集群并开始监听 Pod 事件。你可以通过以下方式测试：

1. 创建一个新的 Pod
2. 修改现有 Pod 的配置
3. 删除一个 Pod

程序会实时打印出这些事件的信息。

## 构建
```bash
make build GOOS=linux GOARCH=amd64 
```

