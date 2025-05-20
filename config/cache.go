package config

var clusters map[string]*Cluster
var namespaces map[string]struct{}
var orgIDs map[uint]string
var nodeGroups map[string]string
var notebooks map[string]string
var trainJobs map[string]string
var inferences map[string]string

var price Price
var config Config

type Cluster struct {
	ID     uint
	Name   string `json:"name"`
	Config string `json:"config"`
}

type Config struct {
	Enable             bool   `json:"enabled"`
	StopPod            bool   `json:"stopPod"`
	CheckAllNamespaces bool   `json:"checkAllNamespaces"`
	SysNamespaces      string `json:"sysNamespaces"`
}

type Price struct {
	CPU     int64            `json:"cpu,omitempty"`
	Memory  int64            `json:"memory,omitempty"`
	GPU     map[string]int64 `json:"gpu,omitempty"`
	Storage map[uint]int64   `json:"storage,omitempty"`
}

func GetClusters() map[string]*Cluster {
	return clusters
}

func GetNamespaces() map[string]struct{} {
	return namespaces
}

func SetNamespaces(ns map[string]struct{}) {
	namespaces = ns
}

func SetClusterInfo(cs map[string]*Cluster) {
	clusters = cs
}

func SetOrgIDs(ids map[uint]string) {
	orgIDs = ids
}

func GetOrgIDs() map[uint]string {
	return orgIDs
}

func SetPrice(p Price) {
	price = p
}

func GetPrice() Price {
	return price
}

func SetNodeGroups(ngs map[string]string) {
	nodeGroups = ngs
}
func GetNodeGroups() map[string]string {
	return nodeGroups
}

func SetNotebooks(nbs map[string]string) {
	notebooks = nbs
}

func GetNotebooks() map[string]string {
	return notebooks
}

func SetTrainJobs(tj map[string]string) {
	trainJobs = tj
}

func GetTrainJobs() map[string]string {
	return trainJobs
}

func SetInferences(infer map[string]string) {
	inferences = infer
}
func GetInferences() map[string]string {
	return inferences
}

func SetConfig(c Config) {
	config = c
}

func GetConfig() Config {
	return config
}
