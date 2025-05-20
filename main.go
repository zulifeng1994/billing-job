package main

import (
	"billing-job/controller"
	"billing-job/models"
	"fmt"
	"net/http"
)

// startHealthCheck 启动健康检查服务
func startHealthCheck() {
	http.HandleFunc("/health", healthCheckHandler)

	// 启动 HTTP 服务
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Failed to start health check server: %v\n", err)
	}
}

// healthCheckHandler 处理健康检查请求
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// 返回健康状态
	w.WriteHeader(http.StatusOK)
}

func main() {
	// 启动健康检查服务
	go startHealthCheck()

	// list cluster from db
	cluster := models.Cluster{}
	cluster.SetClusterConfig()

	controller.BillingMain()
	// 保持主进程运行
	select {}
}
