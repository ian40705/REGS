package service

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"regs/db"
	"regs/model"
)

func setupTestDB(t *testing.T) {
	t.Helper()

	dbFile := filepath.Join(t.TempDir(), "regs-test.db")
	if err := db.Init("file:" + dbFile + "?mode=rwc"); err != nil {
		t.Fatal(err)
	}

	if err := model.Migrate(db.Get()); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.Get().DB(); err == nil {
			sqlDB.Close()
		}
	})
}

func TestResolveJudgeInputAnswerPathsUsesProblemTestData(t *testing.T) {
	setupTestDB(t)

	testcaseZip := filepath.Join(t.TempDir(), "testcases.zip")
	createZip(t, testcaseZip, map[string]string{
		"input.txt":  "3 4\n",
		"answer.txt": "7\n",
	})

	problem := model.Problem{
		Title:        "Sum",
		Description:  "sum two numbers",
		TestcasePath: testcaseZip,
	}
	if err := db.Get().Create(&problem).Error; err != nil {
		t.Fatal(err)
	}

	inputPath, answerPath, err := resolveJudgeInputAnswerPaths(problem.ID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(inputPath); err != nil {
		t.Fatalf("expected input file to exist: %v", err)
	}
	if _, err := os.Stat(answerPath); err != nil {
		t.Fatalf("expected answer file to exist: %v", err)
	}

	if filepath.Base(inputPath) != "input.txt" || filepath.Base(answerPath) != "answer.txt" {
		t.Fatalf("unexpected testcase files: %q, %q", inputPath, answerPath)
	}
}

func TestResolveJudgeInputAnswerPathsFailsForUnknownProblem(t *testing.T) {
	setupTestDB(t)

	_, _, err := resolveJudgeInputAnswerPaths(999)
	if err == nil {
		t.Fatal("expected an error for a missing problem")
	}
	if !strings.Contains(err.Error(), "load problem 999") {
		t.Fatalf("expected load problem error, got: %v", err)
	}
}

func TestResolveJudgeInputAnswerPathsFailsForEmptyTestcasePath(t *testing.T) {
	setupTestDB(t)

	problem := model.Problem{Title: "No testcase", Description: "missing path"}
	if err := db.Get().Create(&problem).Error; err != nil {
		t.Fatal(err)
	}

	_, _, err := resolveJudgeInputAnswerPaths(problem.ID)
	if err == nil {
		t.Fatal("expected an error when testcase path is empty")
	}
	if !strings.Contains(err.Error(), "has no testcase path") {
		t.Fatalf("expected testcase path error, got: %v", err)
	}
}

func TestResolveJudgeInputAnswerPathsUsesDirectoryTestcases(t *testing.T) {
	setupTestDB(t)

	testcaseDir := filepath.Join(t.TempDir(), "testcases")
	if err := os.MkdirAll(testcaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testcaseDir, "input.txt"), []byte("1 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testcaseDir, "answer.txt"), []byte("3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	problem := model.Problem{
		Title:        "Directory testcase",
		Description:  "uses a folder of test data",
		TestcasePath: testcaseDir,
	}
	if err := db.Get().Create(&problem).Error; err != nil {
		t.Fatal(err)
	}

	inputPath, answerPath, err := resolveJudgeInputAnswerPaths(problem.ID)
	if err != nil {
		t.Fatal(err)
	}
	if inputPath != filepath.Join(testcaseDir, "input.txt") || answerPath != filepath.Join(testcaseDir, "answer.txt") {
		t.Fatalf("expected direct directory paths, got %q and %q", inputPath, answerPath)
	}
}

func TestUnzipToRejectsZipSlip(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "evil.zip")
	createZip(t, zipPath, map[string]string{"../escape.txt": "bad"})

	destDir := filepath.Join(t.TempDir(), "out")
	if err := unzipTo(zipPath, destDir); err == nil {
		t.Fatal("expected zip slip path traversal to be rejected")
	}
}

func createZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
}
