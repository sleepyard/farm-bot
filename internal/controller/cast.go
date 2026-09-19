package controller

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/playerlog"
)

// ErrHandScanMissed 三档手牌扫描都未悬停到目标。调用方应恢复决策，不要本回合拉黑该牌。
var ErrHandScanMissed = errors.New("手牌扫描未悬停")

// 1280×720 手牌扫描。步长/停留对齐 Python Controller._CAST_SWEEP_PACING。
const (
	handScanY          = 680 // 原 700 偏低，整条扫描线上移 20px
	handScanX0         = 120
	handScanX1         = 1160
	hoverSettle        = 220 * time.Millisecond // 命中后停稳再确认悬停
	handScanRetryPause = 800 * time.Millisecond
)

type sweepPace struct {
	step  int
	dwell time.Duration
}

// Python: _CAST_SWEEP_PACING = ((10, 0.01), (8, 0.018), (7, 0.024))
var handSweepPacing = []sweepPace{
	{10, 10 * time.Millisecond},
	{8, 18 * time.Millisecond},
	{7, 24 * time.Millisecond},
}

var reObjectID = regexp.MustCompile(`(?i)"objectId"\s*:\s*(\d+)`)

// PlayCard 分两阶段：① 慢速扫描直到最新悬停 objectId==目标 ② 停止扫描并原地双击。
// instanceTag 形如 instance=219||name=Plains。
func (c *Controller) PlayCard(instanceID int, label, instanceTag string) error {
	if instanceID <= 0 {
		return fmt.Errorf("无效 instanceId")
	}
	if c.LogPath == "" {
		return fmt.Errorf("未配置 Player.log，无法扫描悬停")
	}
	if instanceTag == "" {
		instanceTag = fmt.Sprintf("instance=%d||name=?", instanceID)
	}
	c.log(fmt.Sprintf("BOT操作: %s 开始手牌扫描 %s", label, instanceTag))

	reader := &playerlog.Reader{Path: c.LogPath}
	var lastErr error
	var hitX int
	for i, pace := range handSweepPacing {
		if i > 0 {
			c.log(fmt.Sprintf("BOT操作: %s 未悬停，%dms 后重扫 %dpx/%dms %s",
				label, handScanRetryPause.Milliseconds(), pace.step, pace.dwell.Milliseconds(), instanceTag))
			time.Sleep(handScanRetryPause)
		}
		x, err := c.scanHandFor(reader, instanceID, instanceTag, pace)
		if err == nil {
			hitX = x
			lastErr = nil
			break
		}
		lastErr = err
	}
	if lastErr != nil {
		c.dismissStuckHandScan(instanceTag)
		return fmt.Errorf("%w %s", ErrHandScanMissed, instanceTag)
	}

	// 命中后不再 Focus/置顶重定位：Python cast 是原地 left_click×2。
	c.log(fmt.Sprintf("BOT操作: %s 扫描停止，准备双击 %s @(%d,%d)", label, instanceTag, hitX, handScanY))
	time.Sleep(hoverSettle)
	if !c.freshHoverIs(reader, instanceID) {
		return fmt.Errorf("双击前悬停已不是 %s", instanceTag)
	}
	if err := input.DoubleClickClient(c.HWND, hitX, handScanY); err != nil {
		return err
	}
	c.log(fmt.Sprintf("BOT操作: %s 双击已发出 %s @(%d,%d)", label, instanceTag, hitX, handScanY))
	return nil
}

// dismissStuckHandScan 三次扫手牌失败后 Esc → 1s → Esc，关掉放大/菜单遮罩。
func (c *Controller) dismissStuckHandScan(instanceTag string) {
	c.log(fmt.Sprintf("BOT操作: 三次未悬停到 %s，按 Esc", instanceTag))
	input.FocusIfNeeded(c.HWND)
	if err := input.TapEscape(); err != nil {
		c.log("BOT操作: Esc 失败: " + err.Error())
		return
	}
	time.Sleep(3 * time.Second)
	c.log("BOT操作: 间隔 3s 后再按 Esc")
	if err := input.TapEscape(); err != nil {
		c.log("BOT操作: 第二次 Esc 失败: " + err.Error())
	}
}

func (c *Controller) scanHandFor(reader *playerlog.Reader, instanceID int, instanceTag string, pace sweepPace) (int, error) {
	x, _, err := c.scanHandAt(reader, instanceID, instanceTag, pace, handScanY)
	return x, err
}

func (c *Controller) scanHandAt(reader *playerlog.Reader, instanceID int, instanceTag string, pace sweepPace, y int) (int, int, error) {
	step := pace.step
	if step < 1 {
		step = 10
	}
	dwell := pace.dwell
	if dwell <= 0 {
		dwell = 10 * time.Millisecond
	}
	if y < 0 {
		y = 0
	}
	for x := handScanX0; x <= handScanX1; x += step {
		if err := input.MoveClient(c.HWND, x, y); err != nil {
			return 0, 0, err
		}
		base, err := reader.Size()
		if err != nil {
			return 0, 0, err
		}
		time.Sleep(dwell)
		chunk, err := reader.Since(base, 512_000, false)
		if err != nil {
			continue
		}
		last := lastObjectID(chunk)
		if last != instanceID {
			continue
		}
		c.log(fmt.Sprintf("BOT操作: 悬停命中 %s @(%d,%d)，结束扫描", instanceTag, x, y))
		return x, y, nil
	}
	return 0, 0, fmt.Errorf("手牌扫描未悬停到 %s", instanceTag)
}

func (c *Controller) freshHoverIs(reader *playerlog.Reader, instanceID int) bool {
	base, err := reader.Size()
	if err != nil {
		return false
	}
	time.Sleep(80 * time.Millisecond)
	chunk, err := reader.Since(base, 256_000, false)
	if err != nil {
		return false
	}
	// 若本窗口无新悬停，允许沿用刚才扫描命中（不强制）
	if chunk == "" {
		return true
	}
	last := lastObjectID(chunk)
	return last == 0 || last == instanceID
}

func lastObjectID(chunk string) int {
	ms := reObjectID.FindAllStringSubmatch(chunk, -1)
	if len(ms) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(ms[len(ms)-1][1])
	return n
}
