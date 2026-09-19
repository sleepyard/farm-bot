// Package server 提供本地 HTTP 控制台。
//
// 设计：Go 只负责业务 API 与静态页面托管；用户通过浏览器操作。
// 这样可以避开各类 Go GUI 框架在 Windows 上的兼容性问题。
package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/appstate"
	"github.com/flourbrain/mtga-farm-bot/internal/assets"
	"github.com/flourbrain/mtga-farm-bot/internal/carddb"
	"github.com/flourbrain/mtga-farm-bot/internal/hotkey"
	"github.com/flourbrain/mtga-farm-bot/internal/playerlog"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
	"github.com/flourbrain/mtga-farm-bot/internal/windlg"
	"github.com/flourbrain/mtga-farm-bot/internal/window"
)

// 史迹示例小红卡组（assets 根目录固定文件名）。
const historicDeckCodeFile = "史迹卡组小红代码（只打脸）.txt"
const tutorialTextFile = "教程.txt"
const tutorialImageFile = "教程.png"

//go:embed web/*
var webFS embed.FS

// 进程内应用状态（模式等）；后续可换成持久化配置。
var appStore = appstate.NewStore()

// 最近一次成功捕获的 MTGA 会话（供导航使用）。
var (
	lastHWND   uintptr
	lastWidth  int
	lastHeight int
	lastQuests []playerlog.Quest
)

// requestShutdown 由 Run 注入，用于页面「关闭 BOT」。
var requestShutdown func()

// Options 控制本地服务启动参数。
type Options struct {
	// Addr 监听地址，例如 "127.0.0.1:17888"。留空则使用默认端口。
	Addr string
	// OpenBrowser 为 true 时，服务就绪后自动打开系统默认浏览器。
	OpenBrowser bool
}

// Run 启动 HTTP 服务并阻塞，直到 ctx 取消或服务异常退出。
func Run(ctx context.Context, opt Options) error {
	if opt.Addr == "" {
		opt.Addr = "127.0.0.1:17888"
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	requestShutdown = func() {
		script.stop.Store(true)
		cancel()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/capture", handleCapture)
	mux.HandleFunc("/api/cards/load", handleCardsLoad)
	mux.HandleFunc("/api/cards/status", handleCardsStatus)
	mux.HandleFunc("/api/cards/pick-dir", handleCardsPickDir)
	mux.HandleFunc("/api/mode", handleMode)
	mux.HandleFunc("/api/script/start", handleScriptStart)
	mux.HandleFunc("/api/script/stop", handleScriptStop)
	mux.HandleFunc("/api/script/status", handleScriptStatus)
	mux.HandleFunc("/api/script/preview/template", handleScriptPreviewTemplate)
	mux.HandleFunc("/api/script/preview/frame", handleScriptPreviewFrame)
	mux.HandleFunc("/api/dev/assets", handleDevAssets)
	mux.HandleFunc("/api/dev/assets/upload", handleDevAssetUpload)
	mux.HandleFunc("/api/historic/deck-code", handleHistoricDeckCode)
	mux.HandleFunc("/api/tutorial", handleTutorial)
	mux.HandleFunc("/api/tutorial/image", handleTutorialImage)
	mux.HandleFunc("/api/shutdown", handleShutdown)
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		mode := appStore.Mode()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":                  true,
			"game_mode":           mode,
			"mode_label":          appstate.ModeLabel(mode),
			"auto_switch":         appStore.AutoSwitch(),
			"shutdown_after_wins": appStore.ShutdownAfterWins(),
		})
	})

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		return fmt.Errorf("加载前端资源失败: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(static)))

	ln, err := net.Listen("tcp", opt.Addr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", opt.Addr, err)
	}

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  120 * time.Second, // 允许上传较大截图
		WriteTimeout: 600 * time.Second, // 导航含多步模板匹配
	}

	url := "http://" + opt.Addr + "/"
	log.Printf("本地控制台已启动: %s", url)
	log.Printf("全局热键: F9 = 开始/停止脚本")
	if path, err := playerlog.ResolvePath(); err != nil {
		log.Printf("Player.log 尚未就绪: %v", err)
	} else {
		log.Printf("Player.log: %s", path)
	}

	tryStartupCapture()

	if err := hotkey.WatchF9(ctx, toggleScript); err != nil {
		log.Printf("注册 F9 热键失败（仍可用网页按钮）: %v", err)
	} else {
		log.Printf("已注册全局热键 F9（切换开始/停止脚本）")
	}

	if opt.OpenBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			if err := openBrowser(url); err != nil {
				log.Printf("自动打开浏览器失败（可手动访问 %s）: %v", url, err)
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if err == http.ErrServerClosed {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return err
	}
}

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "正在关闭"})
	go func() {
		time.Sleep(200 * time.Millisecond)
		if requestShutdown != nil {
			log.Printf("收到关闭请求，正在退出…")
			requestShutdown()
		}
	}()
}

func handleHistoricDeckCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	abs := filepath.Join(assets.RootDir(), historicDeckCodeFile)
	data, err := os.ReadFile(abs)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": "未找到示例套牌文件: " + historicDeckCodeFile,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"title": "史迹示例套牌代码",
		"text":  strings.TrimRight(string(data), "\r\n"),
	})
}

func handleTutorial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	abs := filepath.Join(assets.RootDir(), tutorialTextFile)
	data, err := os.ReadFile(abs)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": "未找到教程文件: " + tutorialTextFile,
		})
		return
	}
	img := filepath.Join(assets.RootDir(), tutorialImageFile)
	_, imgErr := os.Stat(img)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"title":     "使用教程",
		"text":      strings.TrimRight(string(data), "\r\n"),
		"has_image": imgErr == nil,
	})
}

func handleTutorialImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	abs := filepath.Join(assets.RootDir(), tutorialImageFile)
	if _, err := os.Stat(abs); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, abs)
}

type captureResponse struct {
	OK        bool                `json:"ok"`
	Message   string              `json:"message"`
	Title     string              `json:"title,omitempty"`
	Width     int                 `json:"width,omitempty"`
	Height    int                 `json:"height,omitempty"`
	X         int                 `json:"x,omitempty"`
	Y         int                 `json:"y,omitempty"`
	PlayerLog *playerlog.Snapshot `json:"playerlog,omitempty"`
}

func handleCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result := window.CaptureMTGA()
	resp := captureResponse{
		OK:      result.OK,
		Message: result.Message,
	}
	if result.Candidate != nil {
		resp.Title = result.Candidate.Title
		resp.Width = result.Candidate.ClientRect.W
		resp.Height = result.Candidate.ClientRect.H
		resp.X = result.Candidate.ClientRect.X
		resp.Y = result.Candidate.ClientRect.Y
	}

	if result.OK {
		snap := rememberCapture(result)
		resp.PlayerLog = &snap
	}

	writeJSON(w, http.StatusOK, resp)
}

func handleCardsLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Printf("开始加载卡牌数据…")
	result := carddb.RefreshAndLoad("", carddb.DefaultCacheDir())
	if result.OK {
		log.Printf("卡牌加载完成: %s (source=%s skipped=%v)", result.Message, result.Source, result.Skipped)
	} else {
		log.Printf("卡牌加载失败: %s", result.Message)
	}
	writeJSON(w, http.StatusOK, result)
}

func handleCardsStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, cardsStatusPayload())
}

func cardsStatusPayload() map[string]any {
	raw, err := carddb.ResolveRawDir()
	ok := false
	msg := "尚未找到卡牌数据目录。请点击「选择卡牌数据目录」，选到 MTGA_Data\\Downloads\\Raw。"
	if err == nil && raw != "" {
		ok = true
		msg = "已定位卡牌目录（捕获窗口后会自动加载）。"
		if db := carddb.Global(); db != nil && db.Len() > 0 {
			msg = fmt.Sprintf("卡牌数据已加载：%d 张", db.Len())
		}
	}
	return map[string]any{
		"ok":      ok,
		"raw_dir": raw,
		"saved":   carddb.LoadSavedRawDir(),
		"count": func() int {
			if db := carddb.Global(); db != nil {
				return db.Len()
			}
			return 0
		}(),
		"message": msg,
	}
}

func handleCardsPickDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Printf("弹出卡牌数据目录选择框…")
	picked, err := windlg.PickFolder("选择 MTGA 卡牌数据目录（MTGA_Data\\Downloads\\Raw）")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	if picked == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "canceled": true, "message": "已取消选择目录"})
		return
	}
	raw, err := carddb.NormalizePickedRawDir(picked)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"picked":  picked,
			"message": "该文件夹里没有 Raw_CardDatabase*.mtga。请选到游戏安装目录下的 MTGA_Data\\Downloads\\Raw（也可以先选 MTGA 根目录，程序会往下找）。",
		})
		return
	}
	if err := carddb.SaveRawDir(raw); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "raw_dir": raw, "message": "目录可用，但保存失败: " + err.Error()})
		return
	}
	log.Printf("卡牌数据目录已保存: %s", raw)
	result := carddb.RefreshAndLoad(raw, carddb.DefaultCacheDir())
	if result.OK {
		log.Printf("卡牌加载完成: %s", result.Message)
	} else {
		log.Printf("卡牌加载失败: %s", result.Message)
	}
	writeJSON(w, http.StatusOK, result)
}

type modeResponse struct {
	OK                bool   `json:"ok"`
	GameMode          string `json:"game_mode"`
	ModeLabel         string `json:"mode_label"`
	AutoSwitch        bool   `json:"auto_switch"`
	ShutdownAfterWins bool   `json:"shutdown_after_wins"`
	Message           string `json:"message,omitempty"`
}

func writeModeJSON(w http.ResponseWriter, extra string) {
	mode := appStore.Mode()
	writeJSON(w, http.StatusOK, modeResponse{
		OK:                true,
		GameMode:          mode,
		ModeLabel:         appstate.ModeLabel(mode),
		AutoSwitch:        appStore.AutoSwitch(),
		ShutdownAfterWins: appStore.ShutdownAfterWins(),
		Message:           extra,
	})
}

func handleMode(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeModeJSON(w, "")
	case http.MethodPost:
		var body struct {
			Mode              string `json:"mode"`
			Toggle            bool   `json:"toggle"`
			AutoSwitch        *bool  `json:"auto_switch"`
			ShutdownAfterWins *bool  `json:"shutdown_after_wins"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.AutoSwitch != nil {
			appStore.SetAutoSwitch(*body.AutoSwitch)
		}
		if body.ShutdownAfterWins != nil {
			appStore.SetShutdownAfterWins(*body.ShutdownAfterWins)
		}
		var mode string
		if body.Toggle {
			mode = appStore.Toggle()
		} else if body.Mode != "" {
			mode = appStore.SetMode(body.Mode)
		} else {
			mode = appStore.Mode()
		}
		label := appstate.ModeLabel(mode)
		if body.Toggle || body.Mode != "" {
			log.Printf("排队模式已切换: %s (%s)", mode, label)
			writeModeJSON(w, "当前模式："+label)
			return
		}
		if body.AutoSwitch != nil {
			state := "关闭"
			if *body.AutoSwitch {
				state = "开启"
			}
			log.Printf("自动切换模式: %s", state)
			writeModeJSON(w, "自动切换模式已"+state)
			return
		}
		if body.ShutdownAfterWins != nil {
			state := "关闭"
			if *body.ShutdownAfterWins {
				state = "开启"
			}
			log.Printf("15胜完成后自动关机: %s", state)
			writeModeJSON(w, "15胜完成后自动关机已"+state)
			return
		}
		writeModeJSON(w, "")
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleDevAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := assets.ListStatus()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	ready := 0
	for _, it := range items {
		if it.Exists {
			ready++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"root":    assets.RootDir(),
		"total":   len(items),
		"ready":   ready,
		"missing": len(items) - ready,
		"items":   items,
	})
}

func handleDevAssetUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(12 << 20); err != nil { // 12MB
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "解析上传失败: " + err.Error()})
		return
	}
	path := r.FormValue("path")
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "缺少文件: " + err.Error()})
		return
	}
	defer file.Close()

	if err := assets.SaveUpload(path, file); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error(), "path": path})
		return
	}
	log.Printf("开发者模式上传资源: %s (%s)", path, header.Filename)
	vision.ClearTemplateCache()
	items, _ := assets.ListStatus()
	var item *assets.Item
	for i := range items {
		if strings.EqualFold(items[i].Path, path) {
			item = &items[i]
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "已保存",
		"path":    path,
		"item":    item,
	})
}

func tryStartupCapture() {
	log.Printf("启动时尝试捕获 MTGA…")
	script.appendLog("启动时尝试捕获 MTGA…")
	result := window.CaptureMTGA()
	if !result.OK || result.Candidate == nil {
		msg := result.Message
		if msg == "" {
			msg = "未找到 MTGA 窗口。"
		}
		hint := msg + " 请启动 MTGA（1280×720 窗口模式）后，自行点击「捕获 MTGA」。"
		log.Printf("%s", hint)
		script.appendLog(hint)
		return
	}
	rememberCapture(result)
	log.Printf("%s", result.Message)
	script.appendLog(result.Message)
}

func rememberCapture(result window.CaptureResult) playerlog.Snapshot {
	c := result.Candidate
	lastHWND = c.HWND
	lastWidth = c.ClientRect.W
	lastHeight = c.ClientRect.H
	snap := capturePlayerLogSnapshot()
	lastQuests = snap.Quests
	setScriptQuests(snap.Quests)
	publishCapturePreview(c.HWND)
	return snap
}

func publishCapturePreview(hwnd uintptr) {
	img, err := vision.CaptureClient(hwnd)
	if err != nil {
		log.Printf("捕获成功但截图失败: %v", err)
		script.appendLog("捕获成功但截图失败: " + err.Error())
		return
	}
	script.setGameFrame(img)
}

func capturePlayerLogSnapshot() playerlog.Snapshot {
	return capturePlayerLogSnapshotAt(0)
}

func capturePlayerLogSnapshotAt(floor int64) playerlog.Snapshot {
	path, err := playerlog.ResolvePath()
	if err != nil {
		return playerlog.Snapshot{
			OK:      false,
			Warning: err.Error(),
		}
	}
	s := &playerlog.Snapshotter{
		Reader: &playerlog.Reader{Path: path},
		Floor:  floor,
	}
	return s.Capture()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func openBrowser(url string) error {
	return exec.Command("cmd", "/c", "start", "", url).Start()
}
