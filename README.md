# REGS OJ 助教操作完整指南


## 目錄

1. [環境需求](#1-環境需求)
2. [取得專案並套用修正檔](#2-取得專案並套用修正檔)
3. [準備 submit.zip](#3-準備-submitzip)
4. [Build regs-judge image](#4-build-regs-judge-image)
5. [啟動 API server](#5-啟動-api-server)
6. [執行 test.sh](#6-執行-testsh)
7. [預期輸出](#7-預期輸出)
8. [常見問題排查](#8-常見問題排查)
9. [完全重置環境](#9-完全重置環境)

---

## 1. 環境需求

在跑任何指令前，確認主機已安裝以下工具：

```bash
docker --version          # >= 24.0
docker compose version    # >= 2.0（注意：是 compose 不是 docker-compose）
jq --version              # 任意版本
curl --version            # 任意版本
```

> **Windows 用戶**：請使用 Docker Desktop + Git Bash 或 WSL2 執行所有 shell 指令。不支援 PowerShell / CMD。

---

## 2. 取得專案並套用修正檔

### 2.1 Clone 專案

```bash
git clone <repo-url> REGS-main
cd REGS-main
```

### 2.2 覆蓋三個修正檔

將本次提供的修正檔複製到對應位置：

```bash
# 假設修正檔放在 ~/fixed/ 目錄下
cp ~/fixed/test.sh              ./test.sh
cp ~/fixed/judge.sh             ./judge/judge.sh
cp ~/fixed/build_judge_image.sh ./build_judge_image.sh

# 確保有執行權限
chmod +x test.sh judge/judge.sh build_judge_image.sh
```

> ⚠️ **三個檔案都必須替換**，原始版本有三處 bug 會讓測試完全無法通過。詳見 [修正摘要](#10-修正摘要技術說明)。

### 2.3 確認目錄結構

```
REGS-main/
├── 114-framework/
│   └── 114FinalQ003/          ← 題目資料
│       ├── CMakeLists.txt
│       ├── online-judge/
│       ├── settings.yaml
│       └── solution/
│           ├── Account.h      ← 學生需提交此檔
│           └── entrypoint.cpp ← 學生需提交此檔
├── judge/
│   ├── Dockerfile
│   └── judge.sh               ← 已替換
├── compose.yml
├── Dockerfile
├── build_judge_image.sh       ← 已替換
└── test.sh                    ← 已替換
```

---

## 3. 準備 submit.zip

`test.sh` 預設題目為 `114FinalQ003`，測試時需要一份學生繳交的 zip 檔放在 `/tmp/submit.zip`。

### 方法 A：用 reference solution 打包（推薦，可驗證全 AC）

```bash
cd 114-framework/114FinalQ003/solution
zip /tmp/submit.zip Account.h entrypoint.cpp
cd ../../..
```

### 方法 B：直接使用本次提供的 submit.zip

```bash
cp ~/fixed/submit.zip /tmp/submit.zip
```

> **說明**：`114FinalQ003` 是 ATM Exception Handling 題目。  
> 學生需提交 `Account.h`（實作檔）與 `entrypoint.cpp`（測試框架入口）。  
> CMake build system 會從 `SOURCE_DIR`（student workspace）找這兩個檔案。

---

## 4. Build regs-judge image

Judge container 是由 Go API 動態建立的，每次 submission 建立一個、跑完即消滅。  
在此之前，host 上必須有 `regs-judge:latest` 這個 image。

```bash
./build_judge_image.sh
```

等同於：

```bash
docker build -t regs-judge:latest ./judge
```

### 驗證 image 存在

```bash
docker images | grep regs-judge
```

預期輸出：

```
regs-judge   latest   a1b2c3d4e5f6   2 minutes ago   ~500MB
```

> **何時需要重新 build**：修改了 `judge/judge.sh` 或 `judge/Dockerfile` 之後。平常只需 build 一次。

---

## 5. 啟動 API server

```bash
docker compose up -d --build
```

首次啟動約 **2–4 分鐘**（Go 編譯 + Alpine layer 下載）。

### 確認服務正常

```bash
# 確認 container 在跑
docker ps
```

預期輸出：

```
CONTAINER ID   IMAGE        PORTS                    NAMES
xxxxxxxxxxxx   regs-...     0.0.0.0:18081->8080/tcp  regs-api
```

```bash
# 健康確認：呼叫 problems API，應回傳 HTTP 200
curl -s -o /dev/null -w "%{http_code}" http://localhost:18081/api/problems
# 預期：200
```

> **架構說明**：  
> - `regs-api` container 掛載 `/var/run/docker.sock`，以 Docker-in-Docker（DinD）模式控制宿主機 Docker  
> - 每次 submission 時，API 在宿主機上動態建立 `regs-judge` container 進行評測  
> - 宿主機目錄 `.` 同時映射為 container 內的 `/workspace`，確保路徑一對一對齊

---

## 6. 執行 test.sh

確認以上步驟全部完成後：

```bash
bash test.sh
```

完整流程約 **1–3 分鐘**。

---

## 7. 預期輸出

```
==================================================
   REGS OJ FULL AUTO TEST START
==================================================
[INFO] Checking API server...
[PASS] API alive (200)
[INFO] Testing RBAC...
[PASS] Guest blocked submission
[PASS] Admin login success
[INFO] Testing user login...
[PASS] User login success
[INFO] Testing problem creation...
[PASS] User cannot create problem
[PASS] Problem created
[INFO] Submitting code...
[PASS] Got operatorId
[INFO] Polling judge result...
[3 sec] pending
[6 sec] pending
[9 sec] RUNNING
...
[N sec] AC
[PASS] State finalized
[PASS] Valid status
[PASS] Score valid
[INFO] Checking logs...
[PASS] configure log
[PASS] compile log
[PASS] output log
[INFO] Checking sandbox isolation...
[PASS] Network disabled
[INFO] Testing stats API...
[PASS] Problem stats OK
[PASS] User stats OK
[INFO] Stress testing concurrency...
[PASS] Concurrency test completed
==================================================
PASS: 17
FAIL: 0
==================================================
A+ FULL PASS 🎉
```

---

## 8. 常見問題排查

### API alive FAIL（HTTP 非 200）

```bash
# 查看 API container 日誌
docker logs regs-api --tail 50
```

常見原因：port 18081 被佔用，或 compose 尚未完全啟動。等待幾秒後重試。

---

### State 一直是 `pending` 或 `SE`

```bash
# 確認 regs-judge image 存在
docker images | grep regs-judge

# 確認 API container 能存取 docker.sock
docker inspect regs-api | grep -A2 "Binds"

# 查看 API log 有無 ContainerCreate 錯誤
docker logs regs-api 2>&1 | grep -iE "error|SE|judge|socket"
```

最常見原因：`regs-judge:latest` image 不存在。執行 `./build_judge_image.sh` 後重試。

---

### Network disabled FAIL

```bash
# 手動查最近一個 judge container 的網路設定
CID=$(docker ps -a --filter "ancestor=regs-judge:latest" --format "{{.ID}}" | head -1)
docker inspect "$CID" --format '{{.HostConfig.NetworkMode}}'
# 預期：none
```

若 `$CID` 為空（judge container 已被清除），表示 submission 尚未完成或 image 不存在。

---

### Got operatorId FAIL

```bash
# 確認 /tmp/submit.zip 存在
ls -lh /tmp/submit.zip

# 確認題目 ID 存在（先確認 problem created PASS）
curl -s http://localhost:18081/api/problems | jq .
```

---

### Score valid FAIL（score 為 -1 或非數字）

```bash
# 確認 judge container 執行時有正確輸出 FINAL: N/M
CID=$(docker ps -a --filter "ancestor=regs-judge:latest" --format "{{.ID}}" | head -1)
docker logs "$CID" 2>&1 | tail -5
```

---

## 9. 完全重置環境

若需要從零開始（清除所有資料庫、上傳檔案、image）：

```bash
# 停止並移除所有相關 container 與 volume
docker compose down -v

# 移除 judge image
docker rmi regs-judge:latest

# 清除本地上傳與資料庫
rm -rf uploads/ data/

# 重新 build 並啟動
./build_judge_image.sh
docker compose up -d --build
```

## 快速指令速查

```bash
# ── 完整流程（第一次設定）─────────────────────────────────────

# 1. 套用修正檔
cp ~/fixed/test.sh              ./test.sh
cp ~/fixed/judge.sh             ./judge/judge.sh
cp ~/fixed/build_judge_image.sh ./build_judge_image.sh
chmod +x test.sh judge/judge.sh build_judge_image.sh

# 2. 準備 submit.zip
cd 114-framework/114FinalQ003/solution
zip /tmp/submit.zip Account.h entrypoint.cpp
cd ../../..

# 3. Build judge image
./build_judge_image.sh

# 4. 啟動 API
docker compose up -d --build

# 5. 跑測試
bash test.sh

# ── 重置（清除後重跑）──────────────────────────────────────────
docker compose down -v && docker rmi regs-judge:latest
rm -rf uploads/ data/
./build_judge_image.sh && docker compose up -d --build
bash test.sh
```