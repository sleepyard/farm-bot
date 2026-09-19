package starter

import "github.com/flourbrain/mtga-farm-bot/internal/vision"

// 以下 ROI / 点击点均为 1280×720 客户区坐标（由原版 1920 参考 ×2/3 一次换算，运行时不再缩放）。

var (
	roiHomeAnchor    = vision.ROI{X: 0, Y: 0, W: 507, H: 173}     // home_anchor
	roiHomePlay      = vision.ROI{X: 900, Y: 500, W: 380, H: 220} // play_btn（Home 右下，放宽）
	roiEventsTab     = vision.ROI{X: 767, Y: 27, W: 513, H: 213}  // events_tab
	roiInProgress    = vision.ROI{X: 920, Y: 133, W: 360, H: 333} // in_progress_label
	roiStarterBanner = vision.ROI{X: 13, Y: 53, W: 1000, H: 600}  // starter_deck banner
	roiPlayConfirm   = vision.ROI{X: 773, Y: 453, W: 493, H: 240} // weak play_btn fallback
	roiEventPlay     = vision.ROI{X: 933, Y: 600, W: 347, H: 120} // event_play / submit_deck
	roiViewDeck      = vision.ROI{X: 40, Y: 613, W: 307, H: 107}  // view_deck*
	roiEventTitle    = vision.ROI{X: 13, Y: 67, W: 467, H: 73}    // event_title
	roiRewardClaim   = vision.ROI{X: 967, Y: 567, W: 313, H: 153} // claim
)

var (
	ptHomeTab    = vision.FromBase1920(104, 39)  // 左上 Home 页签（与史迹相同）
	ptDeckBox    = vision.Point{X: 1153, Y: 437} // 当前套牌小盒
	ptSubmitDeck = vision.Point{X: 1153, Y: 671} // Submit Deck 固定点
	ptAllFilter  = vision.Point{X: 1067, Y: 163} // 「全部」筛选参考点
	ptBackArrow  = vision.Point{X: 63, Y: 60}
)

// 套牌网格中心（1280×720），顺序与原版字母表网格一致。
var deckGridCols = []int{122, 317, 511, 706, 902, 1097}
var deckGridRows = []int{257, 467}

var deckGridIndex = map[string]int{
	"WU": 0, "WG": 1, "UB": 2, "UG": 3, "WR": 4, "BG": 5,
	"RG": 6, "BR": 7, "WB": 8, "UR": 9,
}

func deckGridPoint(name string) (vision.Point, bool) {
	idx, ok := deckGridIndex[name]
	if !ok {
		return vision.Point{}, false
	}
	col := idx % 6
	row := idx / 6
	if row > 1 {
		row = 1
	}
	return vision.Point{X: deckGridCols[col], Y: deckGridRows[row]}, true
}
