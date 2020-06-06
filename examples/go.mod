module example

go 1.13

require (
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	k8s.io/klog v0.0.0-00010101000000-000000000000
)

replace k8s.io/klog => ../
