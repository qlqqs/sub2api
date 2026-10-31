package service

import "strings"

const (
	VideoBillingResolution480P  = "480p"
	VideoBillingResolution720P  = "720p"
	VideoBillingResolution1080P = "1080p"
	VideoBillingResolution4K    = "4k"
)

// 视频按秒计费。Seedance 计费上限 60 秒 = 成片 30 秒 + 参考视频 30 秒。
// xAI 未指定时上游默认生成 8 秒。
const (
	VideoBillingMinDurationSeconds     = 1
	VideoBillingMaxDurationSeconds     = 60
	VideoBillingDefaultDurationSeconds = 8
)

// NormalizeVideoBillingDurationSecondsOrDefault 归一化计费用视频时长：
// 未指定（<=0）按上游默认 8 秒计，超出上游允许区间按边界收敛。
func NormalizeVideoBillingDurationSecondsOrDefault(durationSeconds int) int {
	if durationSeconds <= 0 {
		return VideoBillingDefaultDurationSeconds
	}
	if durationSeconds < VideoBillingMinDurationSeconds {
		return VideoBillingMinDurationSeconds
	}
	if durationSeconds > VideoBillingMaxDurationSeconds {
		return VideoBillingMaxDurationSeconds
	}
	return durationSeconds
}

// LookupVideoBillingResolution 归一化分辨率并报告是否为已知档位。
// 配置解析路径用它拒绝未知档位，避免把拼错的高分辨率价格挂到 480p。
func LookupVideoBillingResolution(resolution string) (string, bool) {
	tier := strings.ToLower(strings.TrimSpace(resolution))
	suffix := ""
	if strings.HasSuffix(tier, "-ref") {
		tier = strings.TrimSuffix(tier, "-ref")
		suffix = "-ref"
	}
	var normalized string
	switch tier {
	case "480", "480p", "sd":
		normalized = VideoBillingResolution480P
	case "720", "720p", "hd":
		normalized = VideoBillingResolution720P
	case "1080", "1080p", "full_hd", "full-hd", "fhd":
		normalized = VideoBillingResolution1080P
	case "4k", "2160p", "2160", "uhd":
		normalized = VideoBillingResolution4K
	default:
		return "", false
	}
	return normalized + suffix, true
}

// VideoBillingTier 保留物理分辨率，并按参考视频选择计费档位。
func VideoBillingTier(resolution string, hasReferenceVideo bool) string {
	tier := strings.TrimSuffix(NormalizeVideoBillingResolutionOrDefault(resolution), "-ref")
	if hasReferenceVideo {
		tier += "-ref"
	}
	return tier
}

// NormalizeVideoBillingResolutionOrDefault 用于运行时计费：上游回传的分辨率
// 缺失或无法识别时按最低档兜底，保证请求仍可计费。
func NormalizeVideoBillingResolutionOrDefault(resolution string) string {
	if normalized, ok := LookupVideoBillingResolution(resolution); ok {
		return normalized
	}
	return VideoBillingResolution480P
}
