package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// isValidPath returns if the path is a directory that we are interested in analyzind.
// The vendor directory and hidden directories are examples of directories that we would
// want to skip.
func isValidPath(path string) bool {
	dirs := strings.Split(path, "/")
	for _, d := range dirs {
		if strings.HasPrefix(d, ".") {
			return false
		}
	}
	return true
}

func isTestableDir(dir string) (bool, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), "_test.go") {
			return true, nil
		}
	}
	return false, nil
}

func visit(rootDir string) filepath.WalkFunc {
	return func(path string, f os.FileInfo, err error) error {
		// We only wan't to perform an operation on a directory as a whole
		if !f.IsDir() {
			return nil
		}
		// The diectory shouldn't be in hidden dir, vendor dir, etc.
		if !isValidPath(path) {
			return nil
		}
		// We only care about directories containing go code
		if ok, err := isTestableDir(path); !ok || err != nil {
			return nil
		}

		os.Chdir(filepath.Join(rootDir, path))
		defer os.Chdir(rootDir)
		cmd := exec.Command("go", "test", "-race", "-count=5", "-memprofile", "mem.out", ".")

		out, err := cmd.Output()
		if err != nil {
			fmt.Printf("%10v %v\n", "ERROR", path)
			return nil
		}

		pprofCmd := exec.Command("go", "tool", "pprof", "-list", path+".test", "mem.out")

		out, err = pprofCmd.Output()

		if !strings.Contains(string(out), "Total") {
			fmt.Printf("%10v %v\n", "ERROR", path)
			return nil
		}
		size := strings.TrimSpace(strings.Split(string(out), " ")[1])

		fmt.Printf("%10v %v\n", size, path)

		return nil
	}
}

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Could not get current working directory")
	}
	err = filepath.Walk(".", visit(wd))
}
