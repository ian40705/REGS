package service

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"regs/db"
	"regs/model"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// StartJudge dispatches to the correct judge pipeline based on problem.JudgeType.
//
// JudgeTypeStdio   → judge.sh  <student_dir> <input.txt> <answer.txt> <timelimit>
// JudgeTypeCatch2  → judge.sh  <student_dir> <problem_dir> "catch2" <timelimit>
func StartJudge(operatorID string, zipPath string, problemID uint, timeLimitSec int) {
	timeLimitSec = 30

	var problem model.Problem
	if err := db.Get().First(&problem, problemID).Error; err != nil {
		updateSubmissionStatus(operatorID, "SE")
		return
	}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		updateSubmissionStatus(operatorID, "SE")
		return
	}

	problemDir, err := resolveProblemDir(problem)
	if err != nil {
		updateSubmissionStatus(operatorID, "SE")
		return
	}

	workspace := filepath.Join(filepath.Dir(problemDir), "workspace-"+operatorID)
	_ = os.RemoveAll(workspace)
	_ = os.MkdirAll(workspace, 0o755)

	_ = unzipTo(zipPath, workspace)

	hostProjectPath := os.Getenv("HOST_PROJECT_PATH")

	var hostWorkspacePath, hostProblemDirPath string

	if hostProjectPath != "" {
		relWorkspace, _ := filepath.Rel("/app", workspace)
		relProblem, _ := filepath.Rel("/app", problemDir)

		hostWorkspacePath = filepath.Join(hostProjectPath, relWorkspace)
		hostProblemDirPath = filepath.Join(hostProjectPath, relProblem)
	} else {
		hostWorkspacePath = workspace
		hostProblemDirPath = problemDir
	}

	var binds []string
	var cmd []string

	// mode fix
	mode := "catch2"
	if problem.JudgeType == model.JudgeTypeCMake {
		mode = "cmake"
	}

	switch problem.JudgeType {
	case model.JudgeTypeCatch2, model.JudgeTypeCMake:
		binds = []string{
			hostWorkspacePath + ":/studentcode",
			hostProblemDirPath + ":/problem:ro",
		}

		cmd = []string{
			"/studentcode",
			"/problem",
			mode,
			fmt.Sprintf("%d", timeLimitSec),
		}

	default:
		binds = []string{
			hostWorkspacePath + ":/studentcode",
			hostProblemDirPath + ":/testdata:ro",
		}

		cmd = []string{
			"/studentcode",
			"/testdata/input.txt",
			"/testdata/answer.txt",
			fmt.Sprintf("%d", timeLimitSec),
		}
	}

	hostCfg := &container.HostConfig{
		NetworkMode: "none",
		Binds:       binds,
		Resources: container.Resources{
			NanoCPUs: 1_000_000_000,
			Memory:   1024 * 1024 * 1024,
		},
	}

	resp, err := cli.ContainerCreate(
		context.Background(),
		&container.Config{
			Image:      "regs-judge:latest",
			WorkingDir: "/studentcode",
			Entrypoint: []string{"/workspace/judge.sh"},
			Cmd:        cmd,
		},
		hostCfg, nil, nil, "",
	)

	if err != nil {
		updateSubmissionStatus(operatorID, "SE")
		return
	}

	if err := cli.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		updateSubmissionStatus(operatorID, "SE")
		return
	}

	go waitAndCollect(cli, resp.ID, operatorID, problem, timeLimitSec, timeLimitSec+10)
}

func resolveJudgeInputAnswerPaths(problemID uint) (string, string, error) {
	var problem model.Problem
	if err := db.Get().First(&problem, problemID).Error; err != nil {
		return "", "", fmt.Errorf("load problem %d: %w", problemID, err)
	}
	if strings.TrimSpace(problem.TestcasePath) == "" {
		return "", "", fmt.Errorf("problem %d has no testcase path", problemID)
	}
	dir, err := resolveProblemDir(problem)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "input.txt"), filepath.Join(dir, "answer.txt"), nil
}

// waitAndCollect waits for container exit then parses its stdout to update DB.
func waitAndCollect(
	cli *client.Client,
	containerID string,
	operatorID string,
	problem model.Problem,
	timeLimitSec int,
	stopTimeout int,
) {
	waitCh, errCh := cli.ContainerWait(
		context.Background(),
		containerID,
		container.WaitConditionNotRunning,
	)

	deadline := time.After(time.Duration(timeLimitSec+stopTimeout) * time.Second)

	select {
	case <-waitCh:

	case <-errCh:
		updateSubmissionStatus(operatorID, "SE")
		return

	case <-deadline:
		t := stopTimeout
		_ = cli.ContainerStop(context.Background(), containerID, container.StopOptions{Timeout: &t})
		updateSubmissionStatus(operatorID, "TLE")
		return
	}

	logs, err := cli.ContainerLogs(
		context.Background(),
		containerID,
		container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true, // 🔥 FIX
		},
	)

	if err != nil {
		updateSubmissionStatus(operatorID, "SE")
		return
	}
	defer logs.Close()

	raw, _ := io.ReadAll(logs)
	output := stripDockerLogHeader(raw)

	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		updateSubmissionStatus(operatorID, "SE")
		return
	}

	lastLine := strings.ToUpper(strings.TrimSpace(lines[len(lines)-1]))

	parseAndSave(operatorID, lastLine, problem)
}

func stripDockerLogHeader(raw []byte) string {
	var sb strings.Builder
	for len(raw) >= 8 {
		size := int(raw[4])<<24 | int(raw[5])<<16 | int(raw[6])<<8 | int(raw[7])
		raw = raw[8:]
		if size > len(raw) {
			size = len(raw)
		}
		sb.Write(raw[:size])
		raw = raw[size:]
	}
	return sb.String()
}

func parseAndSave(operatorID string, lastLine string, problem model.Problem) {
	valid := map[string]bool{
		"AC": true,
		"WA": true,
		"CE": true,
		"RE": true,
		"SE": true,
		"TLE": true,
	}

	if strings.HasPrefix(lastLine, "FINAL:") {
		parts := strings.TrimPrefix(lastLine, "FINAL:")
		halves := strings.SplitN(parts, "/", 2)

		got, _ := strconv.Atoi(strings.TrimSpace(halves[0]))
		maxScore := problem.MaxScore

		if len(halves) == 2 {
			if v, err := strconv.Atoi(strings.TrimSpace(halves[1])); err == nil {
				maxScore = v
			}
		}

		status := "WA"
		if maxScore > 0 && got >= maxScore {
			status = "AC"
		}

		updateSubmissionScore(operatorID, status, got)
		return
	}

	if !valid[lastLine] {
		lastLine = "RE"
	}

	updateSubmissionStatus(operatorID, lastLine)
}

func resolveProblemDir(problem model.Problem) (string, error) {
	if strings.TrimSpace(problem.TestcasePath) == "" {
		return "", fmt.Errorf("problem %d has no testcase_path", problem.ID)
	}

	info, statErr := os.Stat(problem.TestcasePath)
	if statErr == nil && info.IsDir() {
		return problem.TestcasePath, nil
	}

	if strings.HasSuffix(strings.ToLower(problem.TestcasePath), ".zip") {
		dest := filepath.Join(os.TempDir(), "regs-problems", fmt.Sprintf("%d", problem.ID))
		_ = os.RemoveAll(dest)
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return "", err
		}
		if err := unzipTo(problem.TestcasePath, dest); err != nil {
			return "", fmt.Errorf("unzip problem: %w", err)
		}
		return dest, nil
	}

	return "", fmt.Errorf("testcase_path %q is neither a directory nor a zip", problem.TestcasePath)
}

func updateSubmissionStatus(operatorID string, status string) error {
	var s model.Submission
	if err := db.Get().Where("operator_id = ?", operatorID).First(&s).Error; err != nil {
		return err
	}
	s.Status = status
	return db.Get().Save(&s).Error
}

func updateSubmissionScore(operatorID string, status string, score int) error {
	var s model.Submission
	if err := db.Get().Where("operator_id = ?", operatorID).First(&s).Error; err != nil {
		return err
	}
	s.Status = status
	s.Score = score
	return db.Get().Save(&s).Error
}

func unzipTo(srcZip, destDir string) error {
	r, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("zip slip: %s", fpath)
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(fpath, f.Mode())
			continue
		}
		_ = os.MkdirAll(filepath.Dir(fpath), 0o755)
		out, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}