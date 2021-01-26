package store

import (
	"fmt"
	"runtime"
)

var s Store

var (
	binaryName = "cf-argo"
	version    = "v99.99.99"
	gitCommit  = ""
)

type Version struct {
	Version   string
	GitCommit string
	GoVersion string
	Platform  string
}

type Store struct {
	BinaryName string
	Version    Version
}

func Get() *Store {
	return &s
}

func init() {
	initVersion()
}

func initVersion() {
	s.Version.Version = version
	s.Version.GitCommit = gitCommit
	s.Version.GoVersion = runtime.Version()
	s.Version.Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}
