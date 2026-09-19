package historic

import "github.com/flourbrain/mtga-farm-bot/internal/vision"

// 1280×720 客户区 ROI（Python 1920×1080 参考 ×2/3，运行时不再缩放）。

var (
	roiHomeAnchor   = vision.ROI{X: 0, Y: 0, W: 507, H: 173}      // home_anchor
	roiHomePlay     = vision.ROI{X: 900, Y: 500, W: 380, H: 220}  // play_btn：Home / 排队共用右下
	roiBladeTabs    = vision.ROI{X: 1033, Y: 27, W: 247, H: 147}  // Events / Find Match
	roiBladeContent = vision.ROI{X: 1033, Y: 120, W: 247, H: 147} // Find Match 面板头
	roiPlaySubtab   = vision.ROI{X: 1033, Y: 167, W: 247, H: 120} // Ranked / Play / Brawl
	// 格式列表略放宽：Historic 行实测中心约 (1135,391)
	roiFormatList  = vision.ROI{X: 1000, Y: 250, W: 280, H: 360}
	roiDecksHeader = vision.ROI{X: 0, Y: 167, W: 533, H: 147} // My Decks 标题
	roiDecksGrid   = vision.ROI{X: 0, Y: 300, W: 533, H: 267} // My Decks 网格 / 「+」
)

// Python _HOME_TAB_POINT_1920 (104,39) → 1280×720 ×2/3。
var ptHomeTab = vision.FromBase1920(104, 39)

// Python _MY_DECKS_FIRST_SLOT_BASE (456,552) @1920 → 1280×720 ×2/3。
var ptFirstDeck = vision.Point{X: 304, Y: 368}

// 排队 Play 固定点（1920 的 1699,996 → ×2/3），模板未命中时用。
var ptQueuePlay = vision.Point{X: 1133, Y: 664}
