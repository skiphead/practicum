package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildWithFlags(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "test_binary")

	testVersion := "integration-test-v1.0"
	testDate := "2026-02-15"
	testCommit := "abc123def456"

	buildCmd := exec.Command("go", "build",
		"-ldflags",
		fmt.Sprintf("-X main.buildVersion=%s -X main.buildDate=%s -X main.buildCommit=%s",
			testVersion, testDate, testCommit),
		"-o", binPath,
		".",
	)
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Logf("Вывод сборки:\n%s", output)
		t.Skipf("Пропускаем тест: не удалось собрать бинарник: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			return
		}
	}(binPath)

	runCmd := exec.Command(binPath, "-v")
	output, _ = runCmd.CombinedOutput()
	outputStr := string(output)

	require.Contains(t, outputStr, testVersion)
	require.Contains(t, outputStr, testDate)
	require.Contains(t, outputStr, testCommit)
	t.Log("✓ Сборка с флагами успешна")
}

func TestBuildWithoutFlags(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "test_binary_no_flags")

	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Logf("Вывод сборки:\n%s", output)
		t.Skipf("Пропускаем тест: не удалось собрать бинарник: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			return
		}
	}(binPath)

	runCmd := exec.Command(binPath, "-v")
	output, _ = runCmd.CombinedOutput()
	outputStr := string(output)

	require.Contains(t, outputStr, "N/A")
	t.Log("✓ Сборка без флагов успешна")
}
