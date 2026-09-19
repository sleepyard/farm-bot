// Package appstate 保存进程内应用状态（模式等），不含各模式的排队业务逻辑。
package appstate

import "sync"

// 与原版 config game_mode 对齐的两个排队模式。
const (
	ModeHistoric = "historic" // 史迹（15胜）
	ModeStarter  = "starter"  // 新手套牌对决（每日任务）
)

// ModeLabel 返回模式的中文显示名。
func ModeLabel(mode string) string {
	if Normalize(mode) == ModeStarter {
		return "新手套牌对决（每日任务）"
	}
	return "史迹（15胜）"
}

// Normalize 校验并规范化模式；非法值回退为 historic。
func Normalize(mode string) string {
	switch mode {
	case ModeStarter:
		return ModeStarter
	case ModeHistoric:
		return ModeHistoric
	default:
		return ModeHistoric
	}
}

// Store 是线程安全的应用状态。
type Store struct {
	mu                sync.RWMutex
	mode              string
	autoSwitch        bool
	shutdownAfterWins bool
	templateLang      string
}

// NewStore 创建状态，默认史迹（与原版一致）。
func NewStore() *Store {
	return &Store{mode: ModeHistoric, templateLang: "en"}
}

// Mode 返回当前模式（historic / starter）。
func (s *Store) Mode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

// AutoSwitch 是否在新手任务完成后自动切到史迹。
func (s *Store) AutoSwitch() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.autoSwitch
}

// SetAutoSwitch 设置自动切换开关。
func (s *Store) SetAutoSwitch(on bool) {
	s.mu.Lock()
	s.autoSwitch = on
	s.mu.Unlock()
}

// ShutdownAfterWins 是否在本会话达到 15 胜后安排关机。
func (s *Store) ShutdownAfterWins() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shutdownAfterWins
}

// SetShutdownAfterWins 设置 15 胜完成后自动关机开关。
func (s *Store) SetShutdownAfterWins(on bool) {
	s.mu.Lock()
	s.shutdownAfterWins = on
	s.mu.Unlock()
}

// TemplateLang returns the template language ("en" = original assets).
func (s *Store) TemplateLang() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.templateLang
}

// SetTemplateLang sets the template language; an empty value resets to "en".
func (s *Store) SetTemplateLang(lang string) {
	if lang == "" {
		lang = "en"
	}
	s.mu.Lock()
	s.templateLang = lang
	s.mu.Unlock()
}

// AutoSwitchTarget 新手模式且每日任务已全部完成时切到史迹，否则保持当前模式。
func AutoSwitchTarget(mode string, autoSwitch, allQuestsDone bool) string {
	if autoSwitch && Normalize(mode) == ModeStarter && allQuestsDone {
		return ModeHistoric
	}
	return Normalize(mode)
}

// SetMode 设置模式；返回规范化后的模式。
func (s *Store) SetMode(mode string) string {
	mode = Normalize(mode)
	s.mu.Lock()
	s.mode = mode
	s.mu.Unlock()
	return mode
}

// Toggle 在 historic ↔ starter 之间切换，返回切换后的模式。
func (s *Store) Toggle() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mode == ModeStarter {
		s.mode = ModeHistoric
	} else {
		s.mode = ModeStarter
	}
	return s.mode
}
