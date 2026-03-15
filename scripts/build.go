package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	appName   = "go-image-server"
	outputDir = "bin"
)

var (
	goosList   = []string{"windows", "linux", "darwin"}
	goarchList = []string{"amd64", "arm64"}
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		printHelp()
		return
	}

	all := len(args) > 0 && (args[0] == "--all" || args[0] == "all")

	if err := os.RemoveAll(outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "clean output dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output dir: %v\n", err)
		os.Exit(1)
	}

	if err := run("go", "mod", "tidy"); err != nil {
		os.Exit(1)
	}

	if all {
		if err := buildAll(); err != nil {
			os.Exit(1)
		}
	} else {
		if err := buildHost(); err != nil {
			os.Exit(1)
		}
	}
}

func printHelp() {
	fmt.Println("Usage:")
	fmt.Println("  go run ./scripts/build.go        # build host binary")
	fmt.Println("  go run ./scripts/build.go --all  # build multi-platform binaries")
}

func buildHost() error {
	fmt.Println("==> Building host binary...")
	output := filepath.Join(outputDir, appName)
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", output, "./cmd/"+appName)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	return runCmd(cmd)
}

func buildAll() error {
	fmt.Println("==> Building multi-platform binaries...")
	for _, goos := range goosList {
		for _, goarch := range goarchList {
			ext := ""
			if goos == "windows" {
				ext = ".exe"
			}
			outName := fmt.Sprintf("%s-%s-%s%s", appName, goos, goarch, ext)
			output := filepath.Join(outputDir, outName)
			fmt.Printf("  - %s/%s -> %s\n", goos, goarch, outName)

			cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", output, "./cmd/"+appName)
			cmd.Env = append(os.Environ(),
				"CGO_ENABLED=0",
				"GOOS="+goos,
				"GOARCH="+goarch,
			)
			if err := runCmd(cmd); err != nil {
				return err
			}
		}
	}
	return nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return runCmd(cmd)
}

func runCmd(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

