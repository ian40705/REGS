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
// JudgeTypeCatch2  → judge_catch2.sh <student_dir> <template_dir> <scores_file> <timelimit>
func StartJudge(operatorID string, zipPath string, problemID uint, timeLimitSec int) {
	var problem model.Problem
	if err := db.Get().First(&problem, problemID).Error; err != nil {
		fmt.Printf("[Judge] load problem error: %v\n", err)
		_ = updateSubmissionStatus(operatorID, "SE")
		return
	}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		fmt.Printf("[Judge] docker client error: %v\n", err)
		_ = updateSubmissionStatus(operatorID, "SE")
		return
	}

	// 解壓學生上傳的 zip 至 workspace
	workspace := filepath.Join(os.TempDir(), "regs-judge-"+operatorID)
	_ = os.RemoveAll(workspace)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		fmt.Printf("[Judge] mkdir workspace error: %v\n", err)
		_ = updateSubmissionStatus(operatorID, "SE")
		return
	}
	if err := unzipTo(zipPath, workspace); err != nil {
		fmt.Printf("[Judge] unzip error: %v\n", err)
	}

	// 取得題目測資目錄（TestcasePath 指向 problem 資料夾，含 template/ 與 online-judge/）
	problemDir, err := resolveProblemDir(problem)
	if err != nil {
		fmt.Printf("[Judge] problem dir resolve error: %v\n", err)
		_ = updateSubmissionStatus(operatorID, "SE")
		return
	}

	var binds []string
	var cmd []string

	switch problem.JudgeType {
	case model.JudgeTypeCatch2:
		// 掛載：
		//   /studentcode  ← 學生上傳的原始碼目錄
		//   /problem      ← 題目目錄（含 template/ spec/ online-judge/ scores.txt）
		scoresFile := filepath.Join(problemDir, "scores.txt")
		if _, err := os.Stat(scoresFile); err != nil {
			fmt.Printf("[Judge] scores.txt not found at %s\n", scoresFile)
			_ = updateSubmissionStatus(operatorID, "SE")
			return
		}
		binds = []string{
			workspace + ":/studentcode",
			problemDir + ":/problem:ro",
		}
		cmd = []string{
			"bash", "/workspace/judge_catch2.sh",
			"/studentcode",          // STUDENT_DIR
			"/problem/template",     // TEMPLATE_DIR
			"/problem/scores.txt",   // SCORES_FILE
			fmt.Sprintf("%d", timeLimitSec),
		}

	default: // JudgeTypeStdio
		inputPath := filepath.Join(problemDir, "input.txt")
		answerPath := filepath.Join(problemDir, "answer.txt")
		binds = []string{
			workspace + ":/studentcode",
			problemDir + ":/testdata:ro",
		}
		cmd = []string{
			"bash", "/workspace/judge.sh",
			"/studentcode",
			"/testdata/input.txt",
			"/testdata/answer.txt",
			fmt.Sprintf("%d", timeLimitSec),
		}
		_ = inputPath
		_ = answerPath
	}

	hostCfg := &container.HostConfig{
		NetworkMode: "none",
		Binds:       binds,
		Resources: container.Resources{
			NanoCPUs: 1_000_000_000, // 1 CPU
			Memory:   512 * 1024 * 1024, // 512 MB
		},
	}

	stopTimeout := timeLimitSec + 10
	resp, err := cli.ContainerCreate(
		context.Background(),
		&container.Config{
			Image:      "regs-judge:latest",
			WorkingDir: "/studentcode",
			Cmd:        cmd,
		},
		hostCfg, nil, nil, "",
	)
	if err != nil {
		fmt.Printf("[Judge] container create error: %v\n", err)
		_ = updateSubmissionStatus(operatorID, "SE")
		return
	}

	if err := cli.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		fmt.Printf("[Judge] container start error: %v\n", err)
		_ = updateSubmissionStatus(operatorID, "SE")
		return
	}

	fmt.Printf("[Judge] started: problem=%d type=%s timeout=%ds\n",
		problemID, problem.JudgeType, timeLimitSec)

	go waitAndCollect(cli, resp.ID, operatorID, problem, timeLimitSec, stopTimeout)
}

// waitAndCollect waits for container exit then parses its stdout to update DB.
func waitAndCollect(cli *client.Client, containerID string, operatorID string,
	problem model.Problem, timeLimitSec int, stopTimeout int) {

	waitCh, errCh := cli.ContainerWait(context.Background(), containerID,
		container.WaitConditionNotRunning)

	deadline := time.After(time.Duration(timeLimitSec+stopTimeout) * time.Second)

	select {
	case <-waitCh:
		// 正常結束
	case <-errCh:
		// 等待出錯，嘗試強制停止
		t := stopTimeout
		_ = cli.ContainerStop(context.Background(), containerID, container.StopOptions{Timeout: &t})
		_ = updateSubmissionStatus(operatorID, "SE")
		return
	case <-deadline:
		// 超過整體上限 → TLE
		t := stopTimeout
		_ = cli.ContainerStop(context.Background(), containerID, container.StopOptions{Timeout: &t})
		_ = updateSubmissionStatus(operatorID, "TLE")
		return
	}

	// 讀 container stdout（judge script 的輸出）
	logs, err := cli.ContainerLogs(context.Background(), containerID,
		container.LogsOptions{ShowStdout: true, ShowStderr: false})
	if err != nil {
		fmt.Printf("[Judge] read logs error: %v\n", err)
		_ = updateSubmissionStatus(operatorID, "SE")
		return
	}
	defer logs.Close()

	raw, _ := io.ReadAll(logs)
	// Docker log stream 前 8 bytes 是 header（stream type + length），需要跳過
	output := stripDockerLogHeader(raw)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	lastLine := strings.TrimSpace(lines[len(lines)-1])

	parseAndSave(operatorID, lastLine, problem)
}

// stripDockerLogHeader 移除 Docker multiplexed log stream 的 8-byte header。
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

// parseAndSave interprets the last line of judge output and updates DB.
func parseAndSave(operatorID string, lastLine string, problem model.Problem) {
	if strings.HasPrefix(lastLine, "SCORED ") {
		// Catch2 模式：SCORED X/Y
		parts := strings.TrimPrefix(lastLine, "SCORED ")
		halves := strings.SplitN(parts, "/", 2)
		got, _ := strconv.Atoi(halves[0])
		maxScore := problem.MaxScore
		if len(halves) == 2 {
			if parsed, err := strconv.Atoi(halves[1]); err == nil {
				maxScore = parsed
			}
		}

		status := "WA"
		if maxScore > 0 && got >= maxScore {
			status = "AC"
		} else if got > 0 {
			status = "PA" // Partial Accept
		}
		_ = updateSubmissionScore(operatorID, status, got)
		fmt.Printf("[Judge] %s → %s (%d/%d)\n", operatorID, status, got, maxScore)
		return
	}

	// Stdio 模式：AC / WA / CE / RE / SE / TLE
	valid := map[string]bool{"AC": true, "WA": true, "CE": true, "RE": true, "SE": true, "TLE": true}
	if !valid[lastLine] {
		lastLine = "SE"
	}
	_ = updateSubmissionStatus(operatorID, lastLine)
	fmt.Printf("[Judge] %s → %s\n", operatorID, lastLine)
}

// resolveProblemDir returns the local path to the problem directory.
// TestcasePath can be:
//   - A directory path  (e.g. /srv/problems/114FinalQ006)
//   - A zip path        (extracted to temp, then returned)
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
