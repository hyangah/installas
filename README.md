# Installas

installas is a tool for building a binary with a fake version stamp.
It is a workaround for [go.dev/issues/50603](https://go.dev/issues/50603).

Note: the checksum is not spoofed, so don't worry.

Usage:

```
   go install github.com/hyangah/installas@latest
   cd <your_project_main_module_directory>
   installas <path_to_your_tool>@<version>
```

For example,

```
$ GOBIN=/tmp/bin installas @v1.0.0

$ go version -m /tmp/bin/installas
/tmp/bin/installas: go1.21.5
        path    github.com/hyangah/installas
        mod     github.com/hyangah/installas    v1.0.0  h1:PJzrQEorpFpFN6+aPTf87Nge8hiROBiX4xUt2SUQNjY=
        dep     golang.org/x/mod        v0.14.0 h1:dGoOF9QVLYng8IHTm7BAyWqCqSheQ5pYWGhzW00YJr0=
        build   -buildmode=exe
        build   -compiler=gc
        build   DefaultGODEBUG=panicnil=1
        ...
```
