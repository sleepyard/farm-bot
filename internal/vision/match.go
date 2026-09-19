package vision

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"sync"
)

// Match 是一次模板匹配命中（客户区局部坐标，中心点）。
type Match struct {
	X, Y  int
	Score float64
}

var (
	tplMu    sync.Mutex
	tplCache = map[string]*image.Gray{}
)

// LoadGray 加载模板为灰度图（带缓存）。
func LoadGray(path string) (*image.Gray, error) {
	tplMu.Lock()
	defer tplMu.Unlock()
	if g, ok := tplCache[path]; ok {
		return g, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("解码 %s: %w", path, err)
	}
	g := toGray(img)
	tplCache[path] = g
	return g, nil
}

// ClearTemplateCache 清空模板缓存（开发者模式上传后可调用）。
func ClearTemplateCache() {
	tplMu.Lock()
	defer tplMu.Unlock()
	tplCache = map[string]*image.Gray{}
}

func toGray(src image.Image) *image.Gray {
	b := src.Bounds()
	dst := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	if rgba, ok := src.(*image.RGBA); ok {
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				i := rgba.PixOffset(b.Min.X+x, b.Min.Y+y)
				r, g, bl := rgba.Pix[i], rgba.Pix[i+1], rgba.Pix[i+2]
				dst.Pix[y*dst.Stride+x] = uint8((int(r)*299 + int(g)*587 + int(bl)*114) / 1000)
			}
		}
		return dst
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := src.At(x, y).RGBA()
			dst.SetGray(x-b.Min.X, y-b.Min.Y, color.Gray{Y: uint8((r*299 + g*587 + bl*114) / (1000 * 256))})
		}
	}
	return dst
}

// FindTemplate 在 haystack（可选 ROI）内做 TM_CCOEFF_NORMED 匹配。
// 返回命中中心（相对整幅 haystack）；未达 threshold 返回 nil。
func FindTemplate(haystack image.Image, templatePath string, roi ROI, threshold float64) (*Match, error) {
	return FindTemplateBest(haystack, templatePath, roi, threshold, nil)
}

// FindTemplateBest 同 FindTemplate，并可选写出未达阈值时的最佳命中。
func FindTemplateBest(haystack image.Image, templatePath string, roi ROI, threshold float64, bestOut **Match) (*Match, error) {
	tpl, err := LoadGray(templatePath)
	if err != nil {
		return nil, err
	}
	return matchGray(haystack, tpl, roi, threshold, bestOut)
}

// FindTemplateMultiScale 在若干尺度上匹配（用于套牌缩略图等）；尺度相对模板本身。
func FindTemplateMultiScale(haystack image.Image, templatePath string, roi ROI, threshold float64, scales []float64) (*Match, error) {
	if len(scales) == 0 {
		return FindTemplate(haystack, templatePath, roi, threshold)
	}
	tpl, err := LoadGray(templatePath)
	if err != nil {
		return nil, err
	}
	var best *Match
	for _, s := range scales {
		if s <= 0 {
			continue
		}
		resized := resizeGray(tpl, s)
		m, err := matchGray(haystack, resized, roi, threshold, nil)
		if err != nil {
			return nil, err
		}
		if m != nil && (best == nil || m.Score > best.Score) {
			cp := *m
			best = &cp
		}
	}
	return best, nil
}

func resizeGray(src *image.Gray, scale float64) *image.Gray {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	nw := max(1, int(math.Round(float64(sw)*scale)))
	nh := max(1, int(math.Round(float64(sh)*scale)))
	dst := image.NewGray(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := min(sh-1, int(float64(y)/scale))
		for x := 0; x < nw; x++ {
			sx := min(sw-1, int(float64(x)/scale))
			dst.SetGray(x, y, src.GrayAt(sx, sy))
		}
	}
	return dst
}

// matchGray 粗到细 NCC：大模板先按步长扫描，再在峰值邻域精修。
// 446×177 横幅在 1000×600 ROI 上从 ~32s 降到亚秒级。
func matchGray(haystack image.Image, tpl *image.Gray, roi ROI, threshold float64, bestOut **Match) (*Match, error) {
	gray := toGray(haystack)
	bounds := gray.Bounds()
	searchROI := roi
	if searchROI.W <= 0 || searchROI.H <= 0 {
		searchROI = ROI{X: bounds.Min.X, Y: bounds.Min.Y, W: bounds.Dx(), H: bounds.Dy()}
	}
	searchROI = ClampROI(searchROI, bounds)
	if searchROI.W <= 0 {
		return nil, nil
	}

	tw, th := tpl.Bounds().Dx(), tpl.Bounds().Dy()
	if tw < 1 || th < 1 || searchROI.W < tw || searchROI.H < th {
		return nil, nil
	}

	nPix := float64(tw * th)
	tplPix := make([]float64, tw*th)
	var tplSum float64
	for y := 0; y < th; y++ {
		off := y * tw
		for x := 0; x < tw; x++ {
			v := float64(tpl.Pix[y*tpl.Stride+x])
			tplPix[off+x] = v
			tplSum += v
		}
	}
	tplMean := tplSum / nPix
	var tplVar float64
	for _, v := range tplPix {
		d := v - tplMean
		tplVar += d * d
	}
	if tplVar < 1e-6 {
		return nil, nil
	}
	tplNorm := math.Sqrt(tplVar)

	step := coarseStep(tw, th)
	bestScore := -1.0
	bestX, bestY := 0, 0 // 左上角

	maxY := searchROI.Y + searchROI.H - th
	maxX := searchROI.X + searchROI.W - tw

	scan := func(x0, y0, x1, y1, st int) {
		if st < 1 {
			st = 1
		}
		if x0 < searchROI.X {
			x0 = searchROI.X
		}
		if y0 < searchROI.Y {
			y0 = searchROI.Y
		}
		if x1 > maxX {
			x1 = maxX
		}
		if y1 > maxY {
			y1 = maxY
		}
		for y := y0; y <= y1; y += st {
			for x := x0; x <= x1; x += st {
				score := nccAt(gray, tplPix, x, y, tw, th, tplMean, tplNorm, nPix)
				if score > bestScore {
					bestScore = score
					bestX, bestY = x, y
				}
			}
		}
	}

	scan(searchROI.X, searchROI.Y, maxX, maxY, step)
	if step > 1 && bestScore >= 0 {
		scan(bestX-step, bestY-step, bestX+step, bestY+step, 1)
	}

	if bestScore < 0 {
		return nil, nil
	}
	best := &Match{X: bestX + tw/2, Y: bestY + th/2, Score: bestScore}
	if bestScore < threshold {
		if bestOut != nil {
			*bestOut = best
		}
		return nil, nil
	}
	return best, nil
}

func coarseStep(tw, th int) int {
	area := tw * th
	switch {
	case area >= 40000:
		return 8
	case area >= 15000:
		return 6
	case area >= 6000:
		return 4
	case area >= 2500:
		return 2
	default:
		return 1
	}
}

// nccAt 单遍计算 TM_CCOEFF_NORMED。
func nccAt(gray *image.Gray, tplPix []float64, x, y, tw, th int, tplMean, tplNorm, nPix float64) float64 {
	var sumH, sumH2, sumHT float64
	for ty := 0; ty < th; ty++ {
		row := gray.Pix[(y+ty)*gray.Stride+x:]
		off := ty * tw
		for tx := 0; tx < tw; tx++ {
			h := float64(row[tx])
			t := tplPix[off+tx]
			sumH += h
			sumH2 += h * h
			sumHT += h * t
		}
	}
	varH := sumH2 - sumH*sumH/nPix
	if varH < 1e-6 {
		return -1
	}
	cross := sumHT - tplMean*sumH
	return cross / (math.Sqrt(varH) * tplNorm)
}
