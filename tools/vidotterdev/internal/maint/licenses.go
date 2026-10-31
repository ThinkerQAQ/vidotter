package maint

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Licenses uses upstream's collector rather than implementing a license parser.
func Licenses(root string) error {
	tools := filepath.Join(root, ".build", "tools")
	if err := ensureDir(tools); err != nil {
		return err
	}
	if err := run(root, []string{"GOBIN=" + tools, "GOTOOLCHAIN=" + runtime.Version()}, "go", "install", "github.com/google/go-licenses/v2@v2.0.1"); err != nil {
		return err
	}
	tool := filepath.Join(tools, "go-licenses")
	if runtime.GOOS == "windows" {
		tool += ".exe"
	}
	for _, target := range targets() {
		output := filepath.Join(root, ".build", "out", "licenses", target.OS+"-"+target.Arch)
		if err := os.RemoveAll(output); err != nil {
			return err
		}
		env := []string{"GOOS=" + target.OS, "GOARCH=" + target.Arch, "CGO_ENABLED=0"}
		if err := run(filepath.Join(root, "bridge"), env, tool, "save", "./cmd/vidotter", "--ignore=vidotter", "--save_path="+output); err != nil {
			return fmt.Errorf("collect %s/%s licenses: %w", target.OS, target.Arch, err)
		}
		report, err := os.Create(filepath.Join(output, "DEPENDENCIES.csv"))
		if err != nil {
			return err
		}
		cmd := exec.Command(tool, "report", "./cmd/vidotter", "--ignore=vidotter")
		cmd.Dir = filepath.Join(root, "bridge")
		cmd.Env = append(os.Environ(), env...)
		cmd.Stdout, cmd.Stderr = report, os.Stderr
		runErr := cmd.Run()
		closeErr := report.Close()
		if runErr != nil {
			return runErr
		}
		if closeErr != nil {
			return closeErr
		}
		if err := copyFile(filepath.Join(runtime.GOROOT(), "LICENSE"), filepath.Join(output, "Go-LICENSE")); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(output, "BUILD.txt"), []byte(runtime.Version()+"\n"+target.OS+"/"+target.Arch+"\nCollector: github.com/google/go-licenses/v2 v2.0.1\n"), 0644); err != nil {
			return err
		}
	}
	return nil
}
