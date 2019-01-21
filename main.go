package main

import (
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
		if d == "vendor" {
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

func visit(rootDir string, debug bool, memLimit int) filepath.WalkFunc {
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

		// Run the command to generate profile data
		cmd := exec.Command("go", "test", "-race", "-count=5", "-memprofile", "mem.out", "./"+path)
		out, err := cmd.CombinedOutput()
		if debug {
			fmt.Printf("go test ./... output: %v\n", string(out))
		}
		if err != nil {
			fmt.Printf("%10v %v\n", "ERROR", path)
			return nil
		}

		// Pares the profile data
		pprofCmd := exec.Command("go", "tool", "pprof", "-list", path+".test", "mem.out")
		out, err = pprofCmd.Output()
		if debug {
			fmt.Printf("go tool pprof output: %v\n", string(out))
		}
		if !strings.Contains(string(out), "Total") {
			fmt.Printf("%10v %v\n", "ERROR", path)
			return nil
		}
		size := strings.TrimSpace(strings.Split(string(out), " ")[1])

		fmt.Printf("%10v %v\n", size, path)

		// If a limit was specified, check to see if the tests passed that limit.
		if memLimit > 0 {
			bytes, err := strToBytes(size)
			if err != nil {
				fmt.Printf("Could not parse unit of size: %v\n", err)
				return nil
			}

			if bytes > memLimit {
				data, err := ioutil.ReadFile("mem.out")
				if err != nil {
					fmt.Printf("Could not read file mem.out\n")
					return nil
				}
				fmt.Println("Base64 encoding of pprof mem.out:\n", base64.StdEncoding.EncodeToString(data))
			}

		}

		return nil
	}
}

const Kilobyte = 1024
const Megabyte = Kilobyte * 1024
const Gigabyte = Megabyte * 1024
const Terabyte = Gigabyte * 1024

// strToBytes converts a united string (ex "10GB") and converts it to the number of bytes.
func strToBytes(s string) (int, error) {
	if len(s) <= 2 {
		return 0, nil
	}
	num, err := strconv.ParseFloat(s[0:len(s)-2], 64)
	if err != nil {
		return 0, err
	}

	unit := strings.ToLower(s[len(s)-2:])
	var t float64
	switch unit {
	case "kb":
		t = Kilobyte
	case "mb":
		t = Megabyte
	case "gb":
		t = Gigabyte
	case "tb":
		t = Terabyte
	default:
		return 0, errors.New("incorrect data unit " + unit)
	}

	return int(num * t), nil
}

func main() {
	debug := flag.Bool("debug", false, "add diagnostic info for debugging")
	limitStr := flag.String("limit", "", "set the limit such that if a test passes it, print the pprof info:")
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Could not get current working directory")
	}

	limit, err := strToBytes(*limitStr)
	if err != nil {
		fmt.Println("Could not parse limit string. Pleas use form (FloatNumber)(TB|GB|MB|KB)")
		return
	}

	err = filepath.Walk(".", visit(wd, *debug, limit))
}
