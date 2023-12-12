// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Installas builds and installs a go binary with a fake version stamp.
// It is a workaround for go.dev/issues/50603.
//
// Usage:
//
//	go install github.com/hyangah/installas@latest
//	cd <your_project_main_module_directory>
//	installas <path_to_your_tool>@<version>
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/zip"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [build flags] <target>\n", filepath.Base(os.Args[0]))
	fmt.Fprintf(os.Stderr, " installs the target package with the specified version.\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, " target: package@version (./cmd/coolbin@v0.0.1) or @version (@v0.0.1)\n")
	fmt.Fprintf(os.Stderr, "The binary will be install in the GOBIN or GOPATH/bin directory.\n")
	fmt.Fprintf(os.Stderr, "If you want to install the binary in a different location, use GOBIN.\n")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	target := strings.TrimSpace(os.Args[len(os.Args)-1])
	targetPath, version, ok := strings.Cut(target, "@")
	if !ok {
		fmt.Fprintf(os.Stderr, "the target should be either package@version or @version\n")
		usage()
		os.Exit(1)
	}
	if !semver.IsValid(version) {
		fmt.Fprintf(os.Stderr, "version %q is invalid\n", version)
		os.Exit(1)
	}
	if targetPath == "" {
		targetPath = "."
	}
	out, err := exec.Command("go", "list", "-f", `{{printf "%s\n%s\n%s" .ImportPath .Module.Path .Module.Dir -}}`, targetPath).Output()
	if err != nil {
		log.Panic(err)
	}
	f := strings.Split(string(out), "\n")
	if len(f) < 3 {
		log.Panicf("unexpected `go list` output:\n%+v", len(f))
	}
	packagePath := strings.TrimSpace(f[0])
	currentModule := strings.TrimSpace(f[1])
	currentModuleRootDir := strings.TrimSpace(f[2])

	rootDir, err := os.MkdirTemp("", "stampinggo")
	if err != nil {
		log.Panic(err)
	}

	if err := writeModuleVersion(rootDir, string(currentModule), version, currentModuleRootDir); err != nil {
		log.Panic(err)
	}

	goproxy := os.Getenv("GOPROXY")
	if goproxy == "" {
		goproxy = "proxy.golang.org,direct"
	}
	if err := os.Setenv("GOPROXY", toURL(rootDir)+","+goproxy); err != nil {
		log.Panic(err)
	}
	gonosumdb := os.Getenv("GONOSUMDB")
	if gonosumdb == "" {
		gonosumdb = os.Getenv("GOPRIVATE")
	}
	if gonosumdb != "" {
		gonosumdb = "," + gonosumdb
	}
	if err := os.Setenv("GONOSUMDB", string(currentModule)+gonosumdb); err != nil {
		log.Panic(err)
	}

	buildFlags := os.Args[1 : len(os.Args)-1]
	args := append([]string{"install"}, buildFlags...)
	args = append(args, fmt.Sprintf("%s@%s", packagePath, version))
	cmd := exec.Command("go", args...)
	fmt.Println("Running ", strings.Join(cmd.Args, " "))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Println(err)
			os.Exit(exitErr.ExitCode())
		}
		log.Panic(err)
	}
}

func checkClose(name string, closer io.Closer, err *error) {
	if cerr := closer.Close(); cerr != nil && *err == nil {
		*err = fmt.Errorf("closing %s: %v", name, cerr)
	}
}

// toURL returns the file uri for a proxy directory.
func toURL(dir string) string {
	// file URLs on Windows must start with file:///. See golang.org/issue/6027.
	path := filepath.ToSlash(dir)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file://" + path
}

func writeModuleVersion(rootDir, mod, ver, sourceDir string) (rerr error) {
	dir := filepath.Join(rootDir, mod, "@v")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "list"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer checkClose("list file", f, &rerr)
	if _, err := f.WriteString(ver + "\n"); err != nil {
		return err
	}

	// Serve the go.mod file on the <version>.mod url, if it exists. Otherwise,
	// serve a stub.
	modContents, err := os.ReadFile(filepath.Join(sourceDir, "go.mod"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, ver+".mod"), modContents, 0644); err != nil {
		return err
	}

	infoContents := []byte(fmt.Sprintf(`{"Version": "%v", "Time":"%v"}`, ver, time.Now().UTC().Format(time.RFC3339)))
	if err := os.WriteFile(filepath.Join(dir, ver+".info"), infoContents, 0644); err != nil {
		return err
	}

	// zip of all the source files.
	zipFile, err := os.OpenFile(filepath.Join(dir, ver+".zip"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer checkClose("zip file", zipFile, &rerr)
	if err := zip.CreateFromDir(zipFile, module.Version{Path: mod, Version: ver}, sourceDir); err != nil {
		return err
	}

	// Populate the /module/path/@latest that is used by @latest query.
	if module.IsPseudoVersion(ver) {
		latestFile := filepath.Join(rootDir, mod, "@latest")
		if err := os.WriteFile(latestFile, infoContents, 0644); err != nil {
			return err
		}
	}
	return nil
}
