package saas

// menu_mapper.go — saas EffectiveMenuNode → lab 契约 MenuNode 递归映射
// （镜像 rails SaasMenuMapper）：saas label 字段是 title（2026-09-14 实测），
// 空 回退 code；icon 空按 type 兜底（group→resource / page→file）；
// 子树按 sortOrder 升序（缺序殿后）。

import (
	"math"
	"sort"
)

// MenuMapper 无状态映射器。
type MenuMapper struct{}

// NewMenuMapper 构造。
func NewMenuMapper() *MenuMapper { return &MenuMapper{} }

// Map 递归映射 saas 菜单树。
func (MenuMapper) Map(roots []any) []map[string]any {
	if len(roots) == 0 {
		return []map[string]any{}
	}
	return mapNodes(sortByOrder(roots))
}

func mapNodes(nodes []any) []map[string]any {
	out := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		src, ok := n.(map[string]any)
		if !ok {
			continue
		}
		node := map[string]any{
			"id":    strOf(src["id"]),
			"label": firstPresence(strOf(src["title"]), strOf(src["code"])),
			"icon":  firstPresence(strOf(src["icon"]), defaultIcon(strOf(src["type"]))),
		}
		if p := strOf(src["path"]); p != "" {
			node["path"] = p
		}
		if children, ok := src["children"].([]any); ok && len(children) > 0 {
			node["children"] = mapNodes(sortByOrder(children))
		}
		out = append(out, node)
	}
	return out
}

// sortByOrder sortOrder 升序（缺序视为 +∞ 殿后，防御性稳定排序）。
func sortByOrder(nodes []any) []any {
	out := append([]any{}, nodes...)
	sort.SliceStable(out, func(i, j int) bool {
		return sortOrderOf(out[i]) < sortOrderOf(out[j])
	})
	return out
}

func sortOrderOf(n any) float64 {
	m, ok := n.(map[string]any)
	if !ok {
		return infinity()
	}
	v, ok := m["sortOrder"].(float64)
	if !ok {
		return infinity()
	}
	return v
}

func infinity() float64 {
	return math.Inf(1)
}

func defaultIcon(typ string) string {
	if typ == "group" {
		return "resource"
	}
	return "file"
}

func strOf(v any) string {
	s, _ := v.(string)
	return s
}

func firstPresence(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
