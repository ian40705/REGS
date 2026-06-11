package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	keysDir := filepath.Join("..", "keys")

	err := os.MkdirAll(keysDir, 0755)
	if err != nil {
		panic(err)
	}

	privPath := filepath.Join(keysDir, "private.pem")
	pubPath := filepath.Join(keysDir, "public.pem")

	// Skip if keys already exist
	if fileExists(privPath) && fileExists(pubPath) {
		fmt.Println("✅ 金鑰已存在，跳過生成。")
		return
	}

	fmt.Println("🔑 生成 EC P-256 金鑰對...")

	// Generate private key
	cmd1 := exec.Command(
		"openssl",
		"genpkey",
		"-algorithm", "EC",
		"-pkeyopt", "ec_paramgen_curve:P-256",
		"-out", privPath,
	)

	cmd1.Stdout = os.Stdout
	cmd1.Stderr = os.Stderr

	if err := cmd1.Run(); err != nil {
		panic(err)
	}

	// Generate public key
	cmd2 := exec.Command(
		"openssl",
		"pkey",
		"-in", privPath,
		"-pubout",
		"-out", pubPath,
	)

	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr

	if err := cmd2.Run(); err != nil {
		panic(err)
	}

	fmt.Printf("✅ 已生成：\n   %s\n   %s\n", privPath, pubPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}